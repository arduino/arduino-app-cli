// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"errors"
	"slices"

	"github.com/arduino/arduino-cli/commands"
	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/arduino/go-paths-helper"
	"github.com/sirupsen/logrus"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

// removePlatformProvidedLibraries unpins the platform-provided libraries, so they follow the platform updates.
func removePlatformProvidedLibraries(ctx context.Context, platform platform.Platform, arduinoApp app.Editable) ([]string, error) {
	sketchPath, ok := arduinoApp.GetSketchPath()
	if !ok {
		return nil, errors.New("no sketch path found in the Arduino app")
	}

	logrus.SetLevel(logrus.ErrorLevel) // Reduce the log level of arduino-cli
	srv := commands.NewArduinoCoreServer()
	if err := SetArduinoCliConfig(ctx, srv); err != nil {
		return nil, err
	}

	var inst *rpc.Instance
	if resp, err := srv.Create(ctx, &rpc.CreateRequest{}); err != nil {
		return nil, err
	} else {
		inst = resp.GetInstance()
	}
	defer func() {
		_, _ = srv.Destroy(ctx, &rpc.DestroyRequest{Instance: inst})
	}()
	if err := srv.Init(
		&rpc.InitRequest{Instance: inst},
		commands.InitStreamResponseToCallbackFunction(ctx, func(r *rpc.InitResponse) error {
			return nil
		}),
	); err != nil {
		return nil, err
	}

	platformLibs, err := platformProvidedLibraries(ctx, srv, inst, platform, sketchPath)
	if err != nil || len(platformLibs) == 0 {
		return nil, err
	}

	libs, err := ListSketchLibraries(ctx, *arduinoApp.App())
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, lib := range libs {
		// A library may already be gone as a dependency of a previous removal.
		if !slices.Contains(platformLibs, lib.Name) || slices.Contains(removed, lib.Name) {
			continue
		}
		removedLibs, err := profileLibRemove(ctx, srv, inst, sketchPath, LibraryReleaseID{Name: lib.Name}, true)
		if err != nil {
			return removed, err
		}
		for _, l := range removedLibs {
			removed = append(removed, l.Name)
		}
	}
	return removed, nil
}

// platformProvidedLibraries returns the names of the libraries that the installed platform
// provides to the sketch. It returns nil when the sketch profile pins the platform version.
func platformProvidedLibraries(ctx context.Context, srv rpc.ArduinoCoreServiceServer, inst *rpc.Instance, platform platform.Platform, sketchPath *paths.Path) ([]string, error) {
	sketchResp, err := srv.LoadSketch(ctx, &rpc.LoadSketchRequest{SketchPath: sketchPath.String()})
	if err != nil {
		return nil, err
	}
	if isPlatformPinned(sketchResp.GetSketch().GetDefaultProfile().GetPlatforms(), platform.PlatformID) {
		return nil, nil
	}

	searchResp, err := srv.PlatformSearch(ctx, &rpc.PlatformSearchRequest{
		Instance:   inst,
		SearchArgs: platform.PlatformID,
	})
	if err != nil {
		return nil, err
	}
	return installedPlatformLibraries(searchResp.GetSearchOutput(), platform.PlatformID), nil
}

func isPlatformPinned(platforms []*rpc.ProfilePlatformReference, platformID string) bool {
	return slices.ContainsFunc(platforms, func(p *rpc.ProfilePlatformReference) bool {
		return p.GetId() == platformID && p.GetVersion() != ""
	})
}

// installedPlatformLibraries returns the libraries declared by the installed release of platformID.
func installedPlatformLibraries(summaries []*rpc.PlatformSummary, platformID string) []string {
	idx := slices.IndexFunc(summaries, func(s *rpc.PlatformSummary) bool {
		return s.GetMetadata().GetId() == platformID
	})
	if idx < 0 {
		return nil
	}
	summary := summaries[idx]
	release, ok := summary.GetReleases()[summary.GetInstalledVersion()]
	if !ok {
		return nil
	}

	var names []string
	for _, dep := range release.GetLibraryDependencies() {
		names = append(names, dep.GetName())
	}
	return names
}
