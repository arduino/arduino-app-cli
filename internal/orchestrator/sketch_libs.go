// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/arduino/arduino-cli/commands"
	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

const indexUpdateInterval = 10 * time.Minute

func AddSketchLibrary(ctx context.Context, app app.ArduinoApp, libRef LibraryReleaseID, addDeps bool) ([]LibraryReleaseID, error) {
	sketchPath, ok := app.GetSketchPath()
	if !ok {
		return nil, errors.New("cannot add a library. Missing sketch folder")
	}

	srv := commands.NewArduinoCoreServer()
	if err := SetArduinoCliConfig(ctx, srv); err != nil {
		return nil, err
	}

	var inst *rpc.Instance
	if res, err := srv.Create(ctx, &rpc.CreateRequest{}); err != nil {
		return nil, err
	} else {
		inst = res.Instance
	}
	defer func() { _, _ = srv.Destroy(ctx, &rpc.DestroyRequest{Instance: inst}) }()
	if err := srv.Init(&rpc.InitRequest{
		Instance: inst,
	}, commands.InitStreamResponseToCallbackFunction(ctx, func(r *rpc.InitResponse) error {
		// TODO: LOG progress/error?
		return nil
	})); err != nil {
		return nil, err
	}

	stream, _ := commands.UpdateLibrariesIndexStreamResponseToCallbackFunction(ctx, func(curr *rpc.DownloadProgress) {
		slog.Debug("downloading library index", "progress", curr.GetMessage())
	})
	// update the local library index after a certain time, to avoid if a library is added to the sketch but the local library index is not update, the compile can fail (because the lib is not found)
	req := &rpc.UpdateLibrariesIndexRequest{Instance: inst, UpdateIfOlderThanSecs: int64(indexUpdateInterval.Seconds())}
	if err := srv.UpdateLibrariesIndex(req, stream); err != nil {
		slog.Warn("error updating library index, skipping", slog.String("error", err.Error()))
	}

	resp, err := srv.ProfileLibAdd(ctx, &rpc.ProfileLibAddRequest{
		Instance:   inst,
		SketchPath: sketchPath.String(),
		Library: &rpc.ProfileLibraryReference{
			Library: &rpc.ProfileLibraryReference_IndexLibrary_{
				IndexLibrary: &rpc.ProfileLibraryReference_IndexLibrary{
					Name:    libRef.Name,
					Version: libRef.Version,
				},
			},
		},
		AddDependencies: &addDeps,
	})
	if err != nil {
		return nil, err
	}

	return f.Map(resp.GetAddedLibraries(), rpcProfileLibReferenceToLibReleaseID), nil
}

func RemoveSketchLibrary(ctx context.Context, app app.ArduinoApp, libRef LibraryReleaseID, removeDeps bool) ([]LibraryReleaseID, error) {
	sketchPath, ok := app.GetSketchPath()
	if !ok {
		return nil, errors.New("cannot remove a library. Missing sketch folder")
	}
	srv := commands.NewArduinoCoreServer()
	if err := SetArduinoCliConfig(ctx, srv); err != nil {
		return nil, err
	}

	var inst *rpc.Instance
	if res, err := srv.Create(ctx, &rpc.CreateRequest{}); err != nil {
		return nil, err
	} else {
		inst = res.Instance
	}
	defer func() { _, _ = srv.Destroy(ctx, &rpc.DestroyRequest{Instance: inst}) }()
	if err := srv.Init(&rpc.InitRequest{
		Instance: inst,
	}, commands.InitStreamResponseToCallbackFunction(ctx, func(r *rpc.InitResponse) error {
		// TODO: LOG progress/error?
		return nil
	})); err != nil {
		return nil, err
	}

	resp, err := srv.ProfileLibRemove(ctx, &rpc.ProfileLibRemoveRequest{
		Instance: inst,
		Library: &rpc.ProfileLibraryReference{
			Library: &rpc.ProfileLibraryReference_IndexLibrary_{
				IndexLibrary: &rpc.ProfileLibraryReference_IndexLibrary{
					Name:    libRef.Name,
					Version: libRef.Version,
				},
			},
		},
		RemoveDependencies: &removeDeps,
		SketchPath:         sketchPath.String(),
	})
	if err != nil {
		return nil, err
	}
	return f.Map(resp.GetRemovedLibraries(), rpcProfileLibReferenceToLibReleaseID), nil
}

func ListSketchLibraries(ctx context.Context, app app.ArduinoApp) ([]LibraryReleaseID, error) {
	sketchPath, ok := app.GetSketchPath()
	if !ok {
		return nil, errors.New("cannot list libraries. Missing sketch folder")
	}

	srv := commands.NewArduinoCoreServer()

	resp, err := srv.ProfileLibList(ctx, &rpc.ProfileLibListRequest{
		SketchPath: sketchPath.String(),
	})
	if err != nil {
		return nil, err
	}

	// Keep only index libraries
	libs := f.Filter(resp.Libraries, func(l *rpc.ProfileLibraryReference) bool {
		return l.GetIndexLibrary() != nil
	})
	return f.Map(libs, rpcProfileLibReferenceToLibReleaseID), nil
}

func rpcProfileLibReferenceToLibReleaseID(ref *rpc.ProfileLibraryReference) LibraryReleaseID {
	l := ref.GetIndexLibrary()
	return LibraryReleaseID{
		Name:         l.GetName(),
		Version:      l.GetVersion(),
		IsDependency: l.GetIsDependency(),
	}
}
