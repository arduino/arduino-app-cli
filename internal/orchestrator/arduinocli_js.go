// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build js

package orchestrator

// The functions of arduinocli.go where there is no arduino-cli, as in a browser: they
// build and flash sketches, and manage the libraries and platforms they use. Each answers
// ErrNoArduinoCli, or nothing.

import (
	"context"
	"io"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/arduino/go-paths-helper"
)

func uploadFirmwareFile(ctx context.Context, verbose bool, fwFile *paths.Path, w io.Writer) (err error) {
	err = ErrNoArduinoCli
	return
}

func initializeArduinoCli(ctx context.Context, sketchPath *paths.Path, w io.Writer) (r0 rpc.ArduinoCoreServiceServer, r1 *rpc.Instance, err error) {
	err = ErrNoArduinoCli
	return
}

func compileSketch(ctx context.Context, srv rpc.ArduinoCoreServiceServer, inst *rpc.Instance, sketchPath, buildPath *paths.Path, platform platform.Platform, fqbn string, verbose bool, w io.Writer) (err error) {
	err = ErrNoArduinoCli
	return
}

func uploadSketch(ctx context.Context, srv rpc.ArduinoCoreServiceServer, inst *rpc.Instance, sketchPath, buildPath *paths.Path, useLegacyFlashToRam bool, fqbn string, w io.Writer) (err error) {
	err = ErrNoArduinoCli
	return
}

func migrateRemoveRouterBridgeIfNeeded(ctx context.Context, platform platform.Platform, app app.Editable) (r0 bool, err error) {
	err = ErrNoArduinoCli
	return
}

func buildSketch(ctx context.Context, appToBuild app.ArduinoApp, platform platform.Platform, buildPath, destPath *paths.Path, verbose bool, cb func(StreamMessage)) (err error) {
	err = ErrNoArduinoCli
	return
}

func legacyUploadSketchInRam(ctx context.Context, w io.Writer, srv rpc.ArduinoCoreServiceServer, inst *rpc.Instance, fqbn string, sketchPath string, buildPath string) (err error) {
	err = ErrNoArduinoCli
	return
}

func configureMicroInRamMode(ctx context.Context, w io.Writer, srv rpc.ArduinoCoreServiceServer, inst *rpc.Instance, fqbn string) (err error) {
	err = ErrNoArduinoCli
	return
}

func GetPlatformMenuOptions(ctx context.Context, platform platform.Platform) (r0 MenuOptions, err error) {
	err = ErrNoArduinoCli
	return
}

func AddSketchLibrary(ctx context.Context, arduinoApp app.Editable, libRef LibraryReleaseID, addDeps bool) (r0 []LibraryReleaseID, err error) {
	err = ErrNoArduinoCli
	return
}

func RemoveSketchLibrary(ctx context.Context, arduinoApp app.Editable, libRef LibraryReleaseID, removeDeps bool) (r0 []LibraryReleaseID, err error) {
	err = ErrNoArduinoCli
	return
}

func ListSketchLibraries(ctx context.Context, app app.ArduinoApp) (r0 []LibraryReleaseID, err error) {
	err = ErrNoArduinoCli
	return
}

func downloadLibsAndPlatformsUsedInExamples(ctx context.Context, cfg config.Configuration, platform platform.Platform, eventCB InitEventCallback) (err error) {
	err = ErrNoArduinoCli
	return
}

func downloadSketchLibsUsedInApp(ctx context.Context, appPath *paths.Path, platform platform.Platform, cli rpc.ArduinoCoreServiceServer, cliInstance *rpc.Instance, downloadProgressCB func(*rpc.DownloadProgress)) (err error) {
	err = ErrNoArduinoCli
	return
}
