// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !js

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"

	"github.com/arduino/arduino-app-cli/internal/fatomic"
	"github.com/arduino/arduino-app-cli/internal/helpers"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-cli/commands"
	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/arduino/go-paths-helper"
	"github.com/sirupsen/logrus"
	"go.bug.st/f"
	semver "go.bug.st/relaxed-semver"
)

func uploadFirmwareFile(
	ctx context.Context,
	verbose bool,
	fwFile *paths.Path,
	w io.Writer,
) error {
	srv, inst, err := initializeArduinoCli(ctx, nil, w)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = srv.Destroy(ctx, &rpc.DestroyRequest{Instance: inst})
	}()

	// Upload the sketch
	uploadServ, _ := commands.UploadFirmwareFileToServerStreams(ctx, w, w)
	if err := srv.UploadFirmwareFile(&rpc.UploadFirmwareFileRequest{
		Instance:     inst,
		FirmwareFile: fwFile.String(),
		Verbose:      verbose,
	}, uploadServ); err != nil {
		return err
	}
	return nil
}

func initializeArduinoCli(ctx context.Context, sketchPath *paths.Path, w io.Writer) (_ rpc.ArduinoCoreServiceServer, _ *rpc.Instance, _err error) {
	logrus.SetLevel(logrus.ErrorLevel) // Reduce the log level of arduino-cli
	srv := commands.NewArduinoCoreServer()
	if err := SetArduinoCliConfig(ctx, srv); err != nil {
		return nil, nil, err
	}

	var inst *rpc.Instance
	if resp, err := srv.Create(ctx, &rpc.CreateRequest{}); err != nil {
		return nil, nil, err
	} else {
		inst = resp.GetInstance()
	}
	defer func() {
		if _err != nil {
			_, _ = srv.Destroy(ctx, &rpc.DestroyRequest{Instance: inst})
		}
	}()

	initReq := &rpc.InitRequest{
		Instance: inst,
	}
	if sketchPath != nil {
		sketchResp, err := srv.LoadSketch(ctx, &rpc.LoadSketchRequest{SketchPath: sketchPath.String()})
		if err != nil {
			return nil, nil, err
		}
		sketch := sketchResp.GetSketch()
		profile := sketch.GetDefaultProfile().GetName()
		if profile == "" {
			return nil, nil, fmt.Errorf("sketch %q has no default profile", sketchPath)
		}

		initReq.Profile = profile
		initReq.SketchPath = sketchPath.String()
	}

	if err := srv.Init(
		initReq,
		commands.InitStreamResponseToCallbackFunction(ctx, func(r *rpc.InitResponse) error {
			var response string
			switch msg := r.GetMessage().(type) {
			case *rpc.InitResponse_InitProgress:
				if progress := msg.InitProgress.GetTaskProgress(); progress != nil {
					response = helpers.ArduinoCLITaskProgressToString(progress)
				}
				if progress := msg.InitProgress.GetDownloadProgress(); progress != nil {
					response = helpers.ArduinoCLIDownloadProgressToString(progress)
				}
			case *rpc.InitResponse_Error:
				response = "Error: " + msg.Error.String()
			case *rpc.InitResponse_Profile:
				response = fmt.Sprintf(
					"Sketch profile configured: Name=%q, Port=%q",
					msg.Profile.GetName(),
					msg.Profile.GetPort(),
				)
			}
			if _, err := w.Write([]byte(response + "\n")); err != nil {
				return err
			}

			return nil
		}),
	); err != nil {
		return nil, nil, err
	}
	return srv, inst, nil
}

func compileSketch(
	ctx context.Context,
	srv rpc.ArduinoCoreServiceServer,
	inst *rpc.Instance,
	sketchPath, buildPath *paths.Path,
	platform platform.Platform,
	fqbn string,
	verbose bool,
	w io.Writer,
) error {
	// build the sketch
	slog.Debug("compile sketch", slog.String("fqbn", fqbn))

	server, getCompileResult := commands.CompilerServerToStreams(ctx, w, w, nil)
	compileReq := rpc.CompileRequest{
		Instance:   inst,
		Fqbn:       fqbn,
		SketchPath: sketchPath.String(),
		BuildPath:  buildPath.String(),
		Jobs:       platform.CompileJobs,
		Verbose:    verbose,
	}
	if err := srv.Compile(&compileReq, server); err != nil {
		return err
	}

	// Output compilations details
	result := getCompileResult()
	f.Assert(result != nil, "Failed to get compilation result")
	// TODO: maybe handle result.GetDiagnostics()
	boardPlatform := result.GetBoardPlatform()
	if boardPlatform != nil {
		_, _ = w.Write([]byte("Board platform: " + boardPlatform.GetId() + " (" + boardPlatform.GetVersion() + ") in " + boardPlatform.GetInstallDir() + "\n"))
	}
	buildPlatform := result.GetBuildPlatform()
	if buildPlatform != nil && buildPlatform.GetInstallDir() != boardPlatform.GetInstallDir() {
		_, _ = w.Write([]byte("Build platform: " + buildPlatform.GetId() + " (" + buildPlatform.GetVersion() + ") in " + buildPlatform.GetInstallDir() + "\n"))

	}
	for _, lib := range result.GetUsedLibraries() {
		_, _ = w.Write([]byte("Used library " + lib.GetName() + " (" + lib.GetVersion() + ") in " + lib.GetInstallDir() + "\n"))
	}
	return nil
}

// uploadSketch flashes what buildPath holds, whether a compile has just written it or
// a release ships it.
func uploadSketch(
	ctx context.Context,
	srv rpc.ArduinoCoreServiceServer,
	inst *rpc.Instance,
	sketchPath, buildPath *paths.Path,
	useLegacyFlashToRam bool,
	fqbn string,
	w io.Writer,
) error {
	// Support the legacy ram upload option if there isn't the new wait_linux_boot option.
	if useLegacyFlashToRam {
		if err := legacyUploadSketchInRam(ctx, w, srv, inst, fqbn, sketchPath.String(), buildPath.String()); err != nil {
			slog.Warn("failed to upload in ram mode, trying to configure the board in ram mode, and retry", slog.String("error", err.Error()))
			if err := configureMicroInRamMode(ctx, w, srv, inst, fqbn); err != nil {
				return err
			}
			return legacyUploadSketchInRam(ctx, w, srv, inst, fqbn, sketchPath.String(), buildPath.String())
		}
		return nil
	}

	stream, _ := commands.UploadToServerStreams(ctx, w, w)
	return srv.Upload(&rpc.UploadRequest{
		Instance:   inst,
		Fqbn:       fqbn,
		SketchPath: sketchPath.String(),
		ImportDir:  buildPath.String(),
	}, stream)
}

// migrateRemoveRouterBridgeIfNeeded removes the Arduino_RouterBridge library from the sketch profile to allow automatic update of the library.
// This is needed by the platform 0.55 will need a new Arduino_RouterBridge library to allow Serial output redirection to Monitor.
// The migration is applied only if the platform in the profile doesn't specify a version.
func migrateRemoveRouterBridgeIfNeeded(ctx context.Context, platform platform.Platform, app app.Editable) (bool, error) {
	logrus.SetLevel(logrus.ErrorLevel) // Reduce the log level of arduino-cli
	srv := commands.NewArduinoCoreServer()
	if err := SetArduinoCliConfig(ctx, srv); err != nil {
		return false, err
	}

	var inst *rpc.Instance
	if resp, err := srv.Create(ctx, &rpc.CreateRequest{}); err != nil {
		return false, err
	} else {
		inst = resp.GetInstance()
	}
	defer func() {
		_, _ = srv.Destroy(ctx, &rpc.DestroyRequest{Instance: inst})
	}()

	sketchPath, ok := app.GetSketchPath()
	if !ok {
		return false, fmt.Errorf("no sketch path found in the Arduino app")
	}
	sketchResp, err := srv.LoadSketch(ctx, &rpc.LoadSketchRequest{SketchPath: sketchPath.String()})
	if err != nil {
		return false, err
	}

	sketch := sketchResp.GetSketch()
	platforms := sketch.GetDefaultProfile().GetPlatforms()
	if slices.ContainsFunc(platforms, func(p *rpc.ProfilePlatformReference) bool {
		return p.GetId() == platform.PlatformID && p.GetVersion() != ""
	}) {
		slog.Debug("skip migration if the platform in the profiles specifies a version")
		return false, nil
	}

	if err := srv.Init(
		&rpc.InitRequest{Instance: inst},
		commands.InitStreamResponseToCallbackFunction(ctx, func(r *rpc.InitResponse) error {
			return nil
		}),
	); err != nil {
		return false, err
	}

	boardInfo, err := srv.BoardDetails(ctx, &rpc.BoardDetailsRequest{
		Instance: inst,
		Fqbn:     platform.FQBN,
	})
	if err != nil {
		return false, err
	}

	platformVersion, err := semver.Parse(boardInfo.GetVersion())
	if err != nil {
		return false, fmt.Errorf("unable to get installed platform version: %w", err)
	}
	slog.Debug("Installed platform version", "version", platformVersion.String())

	if platformVersion.GreaterThan(semver.MustParse("0.54.1")) {
		libs, err := ListSketchLibraries(ctx, *app.App())
		if err != nil {
			return false, fmt.Errorf("unable to list sketch libraries: %w", err)
		}
		if slices.ContainsFunc(libs, func(lib LibraryReleaseID) bool {
			return lib.Name == "Arduino_RouterBridge"
		}) {
			if _, err := RemoveSketchLibrary(ctx, app, LibraryReleaseID{
				Name: "Arduino_RouterBridge",
			}, true); err != nil {
				return false, err
			}
			return true, nil
		} else {
			return false, nil
		}
	}

	return false, nil
}

func buildSketch(ctx context.Context, appToBuild app.ArduinoApp, platform platform.Platform, buildPath, destPath *paths.Path, verbose bool, cb func(StreamMessage)) error {
	output := NewCallbackWriter(func(line string) {
		cb(StreamMessage{data: line})
	})

	sketchPath, ok := appToBuild.GetSketchPath()
	if !ok {
		return fmt.Errorf("no sketch path found in the Arduino app")
	}
	if err := buildPath.MkdirAll(); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	srv, inst, err := initializeArduinoCli(ctx, sketchPath, output)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = srv.Destroy(ctx, &rpc.DestroyRequest{Instance: inst})
	}()

	// The option only exists on the platforms that wait for the Linux side.
	menuOptions, err := GetPlatformMenuOptions(ctx, platform)
	if err != nil {
		slog.Warn("failed to get platform menu options", slog.String("error", err.Error()))
	}
	fqbn := platform.FQBN
	if menuOptions.Has(WaitForApp) {
		fqbn += ":" + WaitForApp.String()
	}

	// Compile the sketch
	if err := compileSketch(
		ctx, srv, inst,
		sketchPath, buildPath,
		platform, fqbn,
		verbose, output,
	); err != nil {
		return err
	}

	// Upload to file
	uploadStream, _ := commands.UploadToServerStreams(ctx, output, output)
	if err := srv.Upload(&rpc.UploadRequest{
		Instance:   inst,
		Fqbn:       fqbn,
		SketchPath: sketchPath.String(),
		ImportDir:  buildPath.String(),
		Verbose:    verbose,
		// There is no board to upload to: "default" is the protocol arduino-cli uses for
		// portless uploads, and it selects the upload.tool.default recipe.
		Port:                 &rpc.Port{Protocol: "default"},
		UploadToFirmwareFile: new(destPath.Join(ReleaseFirmwareFileName).String()),
	}, uploadStream); err != nil {
		return fmt.Errorf("failed to create the sketch artifact: %w", err)
	}

	return nil
}

func legacyUploadSketchInRam(ctx context.Context,
	w io.Writer,
	srv rpc.ArduinoCoreServiceServer,
	inst *rpc.Instance,
	fqbn string,
	sketchPath string,
	buildPath string,
) error {
	upload := func() error {
		stream, _ := commands.UploadToServerStreams(ctx, w, w)
		if err := srv.Upload(&rpc.UploadRequest{
			Instance:   inst,
			Fqbn:       fqbn + ":flash_mode=ram",
			SketchPath: sketchPath,
			ImportDir:  buildPath,
		}, stream); err != nil {
			return err
		}
		return nil
	}
	if err := upload(); err != nil {
		slog.Warn("failed to upload in ram mode, trying to configure the board in ram mode, and retry", slog.String("error", err.Error()))
		if err := configureMicroInRamMode(ctx, w, srv, inst, fqbn); err != nil {
			return err
		}
	}
	return upload()
}

// configureMicroInRamMode uploads an empty binary overing any sketch previously uploaded in flash.
// This is required to be able to upload sketches in ram mode after if there is already a sketch in flash.
func configureMicroInRamMode(
	ctx context.Context,
	w io.Writer,
	srv rpc.ArduinoCoreServiceServer,
	inst *rpc.Instance,
	fqbn string,
) error {
	emptyBinDir := paths.New("/tmp/empty")
	_ = emptyBinDir.MkdirAll()
	defer func() { _ = emptyBinDir.RemoveAll() }()

	zeros, err := os.Open("/dev/zero")
	if err != nil {
		return err
	}
	defer zeros.Close()

	empty, err := emptyBinDir.Join("empty.ino.elf-zsk.bin").Create()
	if err != nil {
		return err
	}
	defer empty.Close()
	if _, err := io.CopyN(empty, zeros, 50); err != nil {
		return err
	}

	stream, _ := commands.UploadToServerStreams(ctx, w, w)
	return srv.Upload(&rpc.UploadRequest{
		Instance:  inst,
		Fqbn:      fqbn + ":flash_mode=flash",
		ImportDir: emptyBinDir.String(),
	}, stream)
}

func GetPlatformMenuOptions(ctx context.Context, platform platform.Platform) (MenuOptions, error) {
	logrus.SetLevel(logrus.ErrorLevel) // Reduce the log level of arduino-cli
	srv := commands.NewArduinoCoreServer()
	if err := SetArduinoCliConfig(ctx, srv); err != nil {
		return MenuOptions{}, err
	}

	var inst *rpc.Instance
	if resp, err := srv.Create(ctx, &rpc.CreateRequest{}); err != nil {
		return MenuOptions{}, err
	} else {
		inst = resp.GetInstance()
	}
	defer func() {
		_, _ = srv.Destroy(ctx, &rpc.DestroyRequest{Instance: inst})
	}()

	if err := srv.Init(
		&rpc.InitRequest{Instance: inst},
		commands.InitStreamResponseToCallbackFunction(ctx, func(r *rpc.InitResponse) error {
			slog.Debug("Arduino init instance", slog.String("instance", r.String()))
			return nil
		}),
	); err != nil {
		return MenuOptions{}, err
	}

	info, err := srv.BoardDetails(ctx, &rpc.BoardDetailsRequest{
		Instance: inst,
		Fqbn:     platform.FQBN,
	})
	if err != nil {
		return MenuOptions{}, err
	}

	var options MenuOptions
	for _, config := range info.GetConfigOptions() {
		option := MenuOption{name: config.GetOption()}
		for _, value := range config.GetValues() {
			option.values = append(option.values, value.GetValue())
		}
		options = append(options, option)
	}
	return options, nil
}

func AddSketchLibrary(ctx context.Context, arduinoApp app.Editable, libRef LibraryReleaseID, addDeps bool) ([]LibraryReleaseID, error) {
	sketchPath, ok := arduinoApp.GetSketchPath()
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

func RemoveSketchLibrary(ctx context.Context, arduinoApp app.Editable, libRef LibraryReleaseID, removeDeps bool) ([]LibraryReleaseID, error) {
	sketchPath, ok := arduinoApp.GetSketchPath()
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

func downloadLibsAndPlatformsUsedInExamples(ctx context.Context, cfg config.Configuration, platform platform.Platform, eventCB InitEventCallback) error {
	// Start an Arduino Core Server RPC server
	logrus.SetOutput(io.Discard) // Suppress logs from Arduino CLI
	var cliInstance *rpc.Instance
	cli := commands.NewArduinoCoreServer()

	if err := SetArduinoCliConfig(ctx, cli); err != nil {
		return fmt.Errorf("could not set Arduino CLI config: %w", err)
	}

	if resp, err := cli.Create(ctx, &rpc.CreateRequest{}); err != nil {
		return fmt.Errorf("could not create Arduino Core Server client: %w", err)
	} else {
		cliInstance = resp.GetInstance()
	}
	defer func() {
		// Close the server instance
		_, _ = cli.Destroy(ctx, &rpc.DestroyRequest{Instance: cliInstance})
	}()

	// Download progress CB
	currLabel := ""
	var reported helpers.LastPercent
	downloadProgressCB := func(curr *rpc.DownloadProgress) {
		if start := curr.GetStart(); start != nil {
			currLabel = start.GetLabel()
			reported = 0
		}
		update := curr.GetUpdate()
		if update == nil || !reported.Moved(update.GetDownloaded(), update.GetTotalSize()) {
			return
		}
		eventCB(InitEvent{Type: InitProgressEvent, Source: InitSourceArduino, Progress: InitProgress{
			Label: currLabel,
			Curr:  update.GetDownloaded(),
			Total: update.GetTotalSize(),
		}})
	}

	// Force-update of the Arduino Libraries index
	{
		str, _ := commands.UpdateLibrariesIndexStreamResponseToCallbackFunction(ctx, downloadProgressCB)
		if err := cli.UpdateLibrariesIndex(&rpc.UpdateLibrariesIndexRequest{Instance: cliInstance}, str); err != nil {
			return fmt.Errorf("could not update libraries index: %w", err)
		}
	}

	// Force-update of the Arduino Platforms index
	{
		str, _ := commands.UpdateIndexStreamResponseToCallbackFunction(ctx, downloadProgressCB)
		if err := cli.UpdateIndex(&rpc.UpdateIndexRequest{Instance: cliInstance}, str); err != nil {
			return fmt.Errorf("could not update platforms index: %w", err)
		}
	}

	// Install the platform, if it is missing
	{
		if err := cli.Init(&rpc.InitRequest{Instance: cliInstance}, commands.InitStreamResponseToCallbackFunction(ctx, func(r *rpc.InitResponse) error {
			if p := r.GetInitProgress().GetDownloadProgress(); p != nil {
				downloadProgressCB(p)
			}
			return nil
		})); err != nil {
			return fmt.Errorf("could not initialize Arduino Core Server: %w", err)
		}

		// Only a missing platform is installed here, and its version is pinned to the
		// configured constraint. An already installed one is left as it is: upgrading
		// it is the job of `system update`.
		version, err := platformVersionToInstall(ctx, cli, cliInstance, platform.PlatformID, cfg.ArduinoPlatformVersionConstraint)
		if err != nil {
			return err
		}
		if version == "" {
			eventCB(InitEvent{Type: InitLogEvent, Source: InitSourceArduino, Message: fmt.Sprintf(
				"platform %s already installed, skipping install", platform.PlatformID)})
		} else {
			platformPackage, architecture, err := platform.PackageAndArchitecture()
			if err != nil {
				return err
			}
			str := commands.PlatformInstallStreamResponseToCallbackFunction(ctx, downloadProgressCB, func(msg *rpc.TaskProgress) {})
			if err := cli.PlatformInstall(&rpc.PlatformInstallRequest{
				Instance:        cliInstance,
				PlatformPackage: platformPackage,
				Architecture:    architecture,
				Version:         version,
			}, str); err != nil {
				return fmt.Errorf("could not install %s platform %s: %w", platform.PlatformID, version, err)
			}
			fatomic.Sync()
		}
	}

	// Get a list of example apps
	pathsToExplore := cfg.ExamplesDirs(platform)
	pathsToExplore.AddAll(cfg.ExamplesAdditionalDirs())
	exampleAppsPath, err := app.FindAppsInFolders(pathsToExplore)
	if err != nil {
		return err
	}

	// After downloading the libs, clean up the download cache
	defer func() {
		_, _ = cli.CleanDownloadCacheDirectory(ctx, &rpc.CleanDownloadCacheDirectoryRequest{Instance: cliInstance})
	}()

	// Download libraries used in each example app
	for _, appPath := range exampleAppsPath {
		if err := downloadSketchLibsUsedInApp(ctx, appPath, platform, cli, cliInstance, downloadProgressCB); err != nil {
			return fmt.Errorf("could not download libs in app %s: %w", appPath, err)
		}
	}
	fatomic.Sync()

	return nil
}

func downloadSketchLibsUsedInApp(ctx context.Context, appPath *paths.Path, platform platform.Platform, cli rpc.ArduinoCoreServiceServer, cliInstance *rpc.Instance, downloadProgressCB func(*rpc.DownloadProgress)) error {
	// Open the app to get the sketch path
	app, err := app.Load(appPath)
	if err != nil {
		return err
	}

	// A release ships a frozen sketch profile, so there is nothing to migrate.
	if editable, err := app.GetAsEditable(); err == nil {
		if ok, err := migrateRemoveRouterBridgeIfNeeded(ctx, platform, editable); err != nil {
			slog.Warn("Failed to migrate app to remove router bridge", "app", appPath, "error", err)
		} else if ok {
			slog.Info("App migrated, RouterBridge has been removed successfully", "app", appPath)
		}
	}

	sketchPath, ok := app.GetSketchPath()
	if !ok {
		return nil
	}

	// Detect the sketch default defaultProfile
	defaultProfile := "default"
	sk, err := cli.LoadSketch(ctx, &rpc.LoadSketchRequest{SketchPath: sketchPath.String()})
	if err != nil {
		return fmt.Errorf("could not load sketch: %w", err)
	}
	if name := sk.GetSketch().GetDefaultProfile().GetName(); name != "" {
		defaultProfile = name
	}

	// Initializing using the profile will force download and install of the missing libraries
	if err := cli.Init(
		&rpc.InitRequest{
			Instance:   cliInstance,
			SketchPath: sketchPath.String(),
			Profile:    defaultProfile,
		},
		commands.InitStreamResponseToCallbackFunction(ctx, func(r *rpc.InitResponse) error {
			if p := r.GetInitProgress().GetDownloadProgress(); p != nil {
				downloadProgressCB(p)
			}
			return nil
		}),
	); err != nil {
		return fmt.Errorf("could not initialize sketch %s: %w", sketchPath.String(), err)
	}

	return nil
}
