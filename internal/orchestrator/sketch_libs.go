// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package orchestrator

import (
	"context"

	"github.com/arduino/arduino-cli/commands"
	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

func AddSketchLibrary(ctx context.Context, app app.ArduinoApp, libRef LibraryReleaseID, addDeps bool) ([]LibraryReleaseID, error) {
	srv := commands.NewArduinoCoreServer()
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

	resp, err := srv.ProfileLibAdd(ctx, &rpc.ProfileLibAddRequest{
		Instance:   inst,
		SketchPath: app.MainSketchPath.String(),
		Library: &rpc.SketchProfileLibraryReference{
			Library: &rpc.SketchProfileLibraryReference_IndexLibrary_{
				IndexLibrary: &rpc.SketchProfileLibraryReference_IndexLibrary{
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

func RemoveSketchLibrary(ctx context.Context, app app.ArduinoApp, libRef LibraryReleaseID) (LibraryReleaseID, error) {
	srv := commands.NewArduinoCoreServer()
	var inst *rpc.Instance
	if res, err := srv.Create(ctx, &rpc.CreateRequest{}); err != nil {
		return LibraryReleaseID{}, err
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
		return LibraryReleaseID{}, err
	}

	resp, err := srv.ProfileLibRemove(ctx, &rpc.ProfileLibRemoveRequest{
		Library: &rpc.SketchProfileLibraryReference{
			Library: &rpc.SketchProfileLibraryReference_IndexLibrary_{
				IndexLibrary: &rpc.SketchProfileLibraryReference_IndexLibrary{
					Name: libRef.Name,
				},
			},
		},
		SketchPath: app.MainSketchPath.String(),
	})
	if err != nil {
		return LibraryReleaseID{}, err
	}
	return rpcProfileLibReferenceToLibReleaseID(resp.GetLibrary()), nil
}

func ListSketchLibraries(ctx context.Context, app app.ArduinoApp) ([]LibraryReleaseID, error) {
	srv := commands.NewArduinoCoreServer()

	resp, err := srv.ProfileLibList(ctx, &rpc.ProfileLibListRequest{
		SketchPath: app.MainSketchPath.String(),
	})
	if err != nil {
		return nil, err
	}

	// Keep only index libraries
	libs := f.Filter(resp.Libraries, func(l *rpc.SketchProfileLibraryReference) bool {
		return l.GetIndexLibrary() != nil
	})
	res := f.Map(libs, func(l *rpc.SketchProfileLibraryReference) LibraryReleaseID {
		return LibraryReleaseID{
			Name:    l.GetIndexLibrary().GetName(),
			Version: l.GetIndexLibrary().GetVersion(),
		}
	})
	return res, nil
}

func rpcProfileLibReferenceToLibReleaseID(ref *rpc.SketchProfileLibraryReference) LibraryReleaseID {
	l := ref.GetIndexLibrary()
	return NewLibraryReleaseID(l.GetName(), l.GetVersion())
}
