// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"slices"

	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/platform"
)

// platformProvidedLibraries returns the names of the libraries that the installed platform
// provides to the sketch. It returns nil when the sketch profile pins the platform version:
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

// isPlatformPinned reports whether the profile platforms fix a version for platformID.
func isPlatformPinned(platforms []*rpc.ProfilePlatformReference, platformID string) bool {
	return slices.ContainsFunc(platforms, func(p *rpc.ProfilePlatformReference) bool {
		return p.GetId() == platformID && p.GetVersion() != ""
	})
}

// installedPlatformLibraries returns the names of the libraries declared by the installed
// release of platformID, or nil if the platform is not installed.
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
