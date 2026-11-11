// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"

	"github.com/arduino/arduino-cli/commands"
	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"
	"github.com/sirupsen/logrus"
	"go.bug.st/f"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/appbook"
	"github.com/arduino/arduino-app-cli/internal/dockerhelper"
	"github.com/arduino/arduino-app-cli/internal/helpers"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/peripherals"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/pipewire"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

var (
	ErrAppAlreadyExists = appbook.ErrAppAlreadyExists
	ErrAppDoesntExists  = appbook.ErrAppDoesntExists
	ErrAppNotFound      = fmt.Errorf("app not found")
	ErrBadRequest       = appbook.ErrBadRequest
	ErrNotPrepared      = fmt.Errorf("app is not prepared")
	ErrAppRunning       = appbook.ErrAppRunning
)

type AppStreamMessage struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type MessageType string

const (
	UnknownType  MessageType = ""
	ProgressType MessageType = "progress"
	InfoType     MessageType = "info"
)

type StreamMessage struct {
	data     string
	progress *Progress
}

type Progress struct {
	Name     string
	Progress float32
}

func (p *StreamMessage) IsData() bool           { return p.data != "" }
func (p *StreamMessage) IsProgress() bool       { return p.progress != nil }
func (p *StreamMessage) GetData() string        { return p.data }
func (p *StreamMessage) GetProgress() *Progress { return p.progress }
func (p *StreamMessage) GetType() MessageType {
	if p.IsData() {
		return InfoType
	}
	if p.IsProgress() {
		return ProgressType
	}
	return UnknownType
}

func StartApp(
	ctx context.Context,
	docker command.Cli,
	provisioner *Provision,
	modelsIndex *modelsindex.ModelsIndex,
	bricksIndex *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
	appToStart app.ArduinoApp,
	cfg config.Configuration,
	platform platform.Platform,
	verbose bool,
	prepare bool,
	cb func(StreamMessage),
) error {
	if cb == nil {
		cb = func(StreamMessage) {}
	}

	bricksIndex = appToStart.Bricks(bricksIndex)

	// A release validates against the models index its build froze, which knows its custom models.
	modelsIndex = func() *modelsindex.ModelsIndex {
		if appToStart.IsRelease() {
			frozen, err := releaseModels(&appToStart, docker, cfg, platform)
			if err != nil {
				slog.Warn("cannot read the models the release ships", slog.String("app", appToStart.Name), slog.String("error", err.Error()))
			} else {
				return frozen
			}
		}
		return modelsIndex
	}()

	if prepare && !appToStart.IsRelease() {
		if err := prepareModels(ctx, docker, appToStart.Descriptor.Bricks, bricksIndex, modelsIndex, platform, cb); err != nil {
			return err
		}
	}

	if err := checkBricks(ctx, appToStart.Descriptor.Bricks, bricksIndex, modelsIndex, true); err != nil {
		return err
	}

	if err := checkPortCollisions(appToStart.Descriptor, bricksIndex, servicesIndex); err != nil {
		return err
	}

	requiredClasses, err := requiredDeviceClasses(bricksIndex, appToStart.Descriptor.Bricks)
	if err != nil {
		return err
	}

	// Detect the board peripherals only when the app needs them.
	var devices peripherals.AvailableDevices
	if len(requiredClasses) > 0 {
		devices, err = peripherals.Detect(ctx, platform)
		if err != nil {
			return err
		}

		if err := checkRequiredDevices(requiredClasses, devices); err != nil {
			return err
		}
	}

	appsStatus, err := getAppsStatus(ctx, docker.Client())
	if err != nil {
		return fmt.Errorf("failed to list apps status: %w", err)
	}

	// Reject if another app is running/starting; otherwise stop any *other*
	// failed app so its sibling services release resources (ports, volumes)
	// that could conflict with the app we're starting.
	for _, s := range appsStatus {
		switch s.Status {
		case StatusRunning, StatusStarting:
			runningApp, err := app.Load(s.AppPath)
			if err != nil {
				return fmt.Errorf("failed to load running app: %w", err)
			}
			return fmt.Errorf("app %q is running", runningApp.Name)
		case StatusFailed:
			if s.AppPath.EqualsTo(appToStart.FullPath) {
				continue
			}
			failedApp, err := app.Load(s.AppPath)
			if err != nil {
				slog.Warn("skipping failed app: cannot load", slog.String("path", s.AppPath.String()), slog.String("error", err.Error()))
				continue
			}
			slog.Debug("stopping failed app before starting a new one", slog.String("name", failedApp.Name))
			if err := StopApp(ctx, docker, platform, failedApp, cfg, nil); err != nil {
				slog.Warn("failed to stop failed app", slog.String("name", failedApp.Name), slog.String("error", err.Error()))
			}
		}
	}

	cb(StreamMessage{data: fmt.Sprintf("Starting app %q", appToStart.Name)})

	// An app installed from a release holds in .cache what a start generates: the
	// compose files and the python env, frozen for the board it was built for.
	release, isRelease := appToStart.GetRelease()
	if isRelease {
		slog.Debug("starting an app installed from a release", slog.String("release", release.ID), slog.String("target", release.Target))
	}

	// We start PW for any platform or addon in order to be consistend with Network and SBC mode.
	if err := pipewire.EnsurePipewireRunning(ctx, cfg); err != nil {
		slog.Warn("failed to enable audio service linger", slog.String("error", err.Error()))
	}

	if err := setLedsToUserControlledMode(platform); err != nil {
		slog.Debug("unable to set status leds", slog.String("error", err.Error()))
	}

	sketchCallbackWriter := NewCallbackWriter(func(line string) {
		cb(StreamMessage{data: line})
	})

	cb(StreamMessage{progress: &Progress{Name: "preparing", Progress: 0.0}})

	if _, ok := appToStart.GetSketchPath(); ok {
		if isRelease {
			cb(StreamMessage{progress: &Progress{Name: "uploading sketch", Progress: 0.0}})

			// The firmware of a release is the one the build compiled: a start only uploads it.
			fwFile := appToStart.ProvisioningStateDir().Join(ReleaseFirmwareFileName)
			if err := uploadFirmwareFile(ctx, verbose, fwFile, sketchCallbackWriter); err != nil {
				return err
			}
		} else {
			cb(StreamMessage{progress: &Progress{Name: "sketch compiling and uploading", Progress: 0.0}})

			if editable, err := appToStart.GetAsEditable(); err == nil {
				if ok, err := migrateRemoveRouterBridgeIfNeeded(ctx, platform, editable); err != nil {
					cb(StreamMessage{data: "Failed to apply app migration for platform arduino:zephyr >0.54.1. Error: " + err.Error()})
				} else if ok {
					cb(StreamMessage{data: "Applied app migration for platform arduino:zephyr >0.54.1. Arduino_RouterBridge is now part of the platform and shouldn't be explicitly specified"})
				}
			}

			if err := compileUploadSketch(ctx, verbose, platform, appToStart, sketchCallbackWriter); err != nil {
				return err
			}
		}

		cb(StreamMessage{progress: &Progress{Name: "sketch updated", Progress: 10.0}})
	}

	if appToStart.MainPythonFile != nil {
		appEnv := appEnvironment(ctx, appToStart, bricksIndex, modelsIndex, platform)

		cb(StreamMessage{data: "python provisioning"})
		provisionStartProgress := float32(0.0)
		if _, ok := appToStart.GetSketchPath(); ok {
			provisionStartProgress = 10.0
		}

		cb(StreamMessage{progress: &Progress{Name: "python provisioning", Progress: provisionStartProgress}})

		// The compose files of a release are the ones the build resolved for the
		// target board: resolving them again here would replace them.
		if !isRelease {
			// An app is provisioned every time it is started: it is editable, so its
			// bricks, model or ports may have changed since the last run.
			if err := provisioner.Resolve(&appToStart, appToStart.ProvisioningStateDir(), bricksIndex, servicesIndex, cfg, appEnv, platform); err != nil {
				return err
			}
		}

		// What the template references, answered on this board. The app half is in
		// the template already, frozen, when the app comes from a release.
		env := hostEnvironment(ctx, appToStart.FullPath, cfg).Merge(appEnv)
		prj, err := provisioner.Render(ctx, &appToStart, env, appToStart.Secrets(bricksIndex))
		if err != nil {
			return err
		}

		// Without a prepare, a start never fetches nor builds an image: it fails on a
		// missing one and asks for a prepare first.
		if !prepare {
			if err := checkImages(ctx, docker, prj); err != nil {
				return err
			}
			for name, service := range prj.Services {
				service.PullPolicy = types.PullPolicyNever
				prj.Services[name] = service
			}
		}

		cb(StreamMessage{data: "python downloading"})

		if err := dockerhelper.PullImages(ctx, docker.Client(), dockerhelper.ComposeImages(prj),
			func(line string) { cb(StreamMessage{data: line}) },
			func(label string, curr, total int64) {
				// Downloading the images is from 20% to 80% of the start of an app.
				cb(StreamMessage{progress: &Progress{Name: label, Progress: 20 + float32(curr*60/total)}})
			}); err != nil {
			return err
		}

		slog.Debug("starting app", slog.String("project", prj.Name))
		line := func(line string) { cb(StreamMessage{data: line}) }
		err = dockerhelper.ComposeUp(ctx, docker, prj, line)
		if errors.Is(err, dockerhelper.ErrNetwork) {
			// The board is likely out of subnets: the apps give up the networks they
			// keep, and this app is started once more.
			cb(StreamMessage{data: "Could not create the network, freeing the ones the apps keep"})
			appContainers, pruneErr := dockerhelper.Containers(ctx, docker.Client(), DockerAppLabel+"=true")
			if pruneErr != nil {
				slog.Warn("failed to list the app containers", slog.String("error", pruneErr.Error()))
			}
			freed, pruneErr := dockerhelper.PruneNetworks(ctx, docker.Client(), ourNetworks(appContainers))
			if pruneErr != nil {
				slog.Warn("failed to free the networks of the apps", slog.String("error", pruneErr.Error()))
			}
			slog.Debug("freed app networks", slog.Int("count", freed))
			err = dockerhelper.ComposeUp(ctx, docker, prj, line)
		}
		if err != nil {
			return err
		}
	}
	cb(StreamMessage{progress: &Progress{Name: "", Progress: 100.0}})
	return nil
}

func stopAppWithCmd(ctx context.Context, docker command.Cli, platform platform.Platform, app app.ArduinoApp, cfg config.Configuration, cmd string, cb func(StreamMessage)) error {
	if cb == nil {
		cb = func(StreamMessage) {}
	}

	switch cmd {
	case "stop":
		cb(StreamMessage{data: fmt.Sprintf("Stopping app %q", app.Name)})
	case "down":
		cb(StreamMessage{data: fmt.Sprintf("Destroying  app %q", app.Name)})
	}

	if err := restoreLedsState(platform); err != nil {
		slog.Debug("unable to set status leds", slog.String("error", err.Error()))
	}

	if err := pipewire.StopIfNotNeeded(ctx, cfg); err != nil {
		slog.Debug("unable to disable audio service linger", slog.String("error", err.Error()))
	}

	if _, ok := app.GetSketchPath(); ok {
		// Before stopping the microcontroller we want to make sure that the app was running.
		running, err := getRunningApp(ctx, docker.Client())
		if err != nil {
			return err
		}
		if running != nil && running.FullPath.String() == app.FullPath.String() {
			cb(StreamMessage{data: "Stopping microcontroller..."})
			// Reset() triggers a reset of the microcontroller.
			// Note: If the sketch was compiled with 'Wait for App', the microcontroller
			// will stop and wait for the wait-for-app signal before restarting the sketch.
			// In all other boot modes (e.g., 'Wait for Linux' or 'Immediate'), the
			// sketch will simply restart.
			if err := platform.GetMicro().Reset(); err != nil {
				slog.Warn("failed to reset microcontroller", slog.String("error", err.Error()))
			}
		}
	}

	if app.MainPythonFile != nil {
		// The project name stops the app without its compose file, which an update
		// can remove, and finds what an older cli started just the same.
		projectName := composeProjectName(app.FullPath, cfg.AppsDir())
		line := func(line string) { cb(StreamMessage{data: line}) }
		var err error
		switch cmd {
		case "down":
			err = dockerhelper.ComposeDown(ctx, docker, projectName, line)
		default:
			err = dockerhelper.ComposeStop(ctx, docker, projectName, line)
		}
		if err != nil {
			return err
		}
	}
	cb(StreamMessage{progress: &Progress{Name: "", Progress: 100.0}})
	return nil
}

func StopApp(ctx context.Context, dockerClient command.Cli, platform platform.Platform, app app.ArduinoApp, cfg config.Configuration, cb func(StreamMessage)) error {
	return stopAppWithCmd(ctx, dockerClient, platform, app, cfg, "stop", cb)
}

func StopAndDestroyApp(ctx context.Context, dockerClient command.Cli, platform platform.Platform, app app.ArduinoApp, cfg config.Configuration, cb func(StreamMessage)) error {
	if err := stopAppWithCmd(ctx, dockerClient, platform, app, cfg, "down", cb); err != nil {
		return err
	}
	if err := cleanAppCacheFiles(app, cb); err != nil {
		return err
	}
	return nil
}

func cleanAppCacheFiles(app app.ArduinoApp, cb func(StreamMessage)) error {
	if cb == nil {
		cb = func(StreamMessage) {}
	}

	// The .cache of an app installed from a release is what the release froze:
	// nothing regenerates it, so a destroy leaves it where it is.
	if app.IsRelease() {
		cb(StreamMessage{data: "Keeping the cache the release ships."})
		return nil
	}

	cachePath := app.FullPath.Join(".cache")

	if exists, _ := cachePath.ExistCheck(); !exists {
		cb(StreamMessage{data: "No cache to clean."})
		return nil
	}
	cb(StreamMessage{data: "Removing app cache files..."})
	slog.Debug("removing app cache", slog.String("path", cachePath.String()))
	if err := cachePath.RemoveAll(); err != nil {
		return fmt.Errorf("unable to remove app cache: %w", err)
	}
	cb(StreamMessage{data: "Cache removed successfully."})
	return nil
}

func RestartApp(
	ctx context.Context,
	docker command.Cli,
	provisioner *Provision,
	modelsIndex *modelsindex.ModelsIndex,
	bricksIndex *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
	appToStart app.ArduinoApp,
	cfg config.Configuration,
	platform platform.Platform,
	verbose bool,
	cb func(StreamMessage),
) error {
	runningApp, err := getRunningApp(ctx, docker.Client())
	if err != nil {
		return err
	}

	if runningApp != nil {
		if runningApp.FullPath.String() != appToStart.FullPath.String() {
			return fmt.Errorf("another app %q is running", runningApp.Name)
		}

		if err := StopApp(ctx, docker, platform, *runningApp, cfg, cb); err != nil {
			return err
		}
	}

	return StartApp(ctx, docker, provisioner, modelsIndex, bricksIndex, servicesIndex, appToStart, cfg, platform, verbose, !appToStart.IsRelease(), cb)
}

func StartDefaultApp(
	ctx context.Context,
	docker command.Cli,
	provisioner *Provision,
	modelsIndex *modelsindex.ModelsIndex,
	bricksIndex *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
	idProvider *appid.Provider,
	cfg config.Configuration,
	platform platform.Platform,
) error {
	app, err := GetDefaultApp(cfg)
	if err != nil {
		return fmt.Errorf("failed to get default app: %w", err)
	}
	if app == nil {
		// default app not set.
		return nil
	}

	status, err := AppDetails(ctx, docker, *app, bricksIndex, idProvider, cfg)
	if err != nil {
		return fmt.Errorf("failed to get app details: %w", err)
	}
	if status.Status == "running" {
		return nil
	}

	// TODO: we need to stop all other running app before starting the default app.
	if err := StartApp(ctx, docker, provisioner, modelsIndex, bricksIndex, servicesIndex, *app, cfg, platform, false, !app.IsRelease(), func(sm StreamMessage) {}); err != nil {
		return fmt.Errorf("failed to start app: %w", err)
	}

	return nil
}

// ListApps lists the catalog with the status of each app.
func ListApps(
	ctx context.Context,
	docker command.Cli,
	req ListAppRequest,
	idProvider *appid.Provider,
	bricksIndex *bricksindex.BricksIndex,
	cfg config.Configuration,
	platform platform.Platform,
) (ListAppResult, error) {
	res, err := appbook.ListApps(req, idProvider, bricksIndex, cfg, platform)
	if err != nil {
		return res, err
	}

	// Get the status of all apps (it will return only the apps that have been started at least once)
	appsStatus, err := AppsStatus(ctx, docker)
	if err != nil {
		slog.Error("unable to get running app", slog.String("error", err.Error()))
	}
	for i, a := range res.Apps {
		res.Apps[i].Status = StatusUninitialized
		if status, ok := appsStatus[a.ID.ToPath().String()]; ok {
			res.Apps[i].Status = status
		}
	}
	return res, nil
}

// ListActiveApps returns all the apps that have a non-uninitialized status in Docker,
// i.e. all apps that have been started at least once, including apps located outside the
// standard apps directory and apps whose metadata is broken or missing on disk.
// The result is sorted by app path.
func ListActiveApps(
	ctx context.Context,
	docker command.Cli,
	idProvider *appid.Provider,
) ([]AppInfo, error) {
	// The statuses returned by getAppsStatus come only from Docker containers that have
	// been started, so StatusUninitialized is never present.
	appsStatus, err := getAppsStatus(ctx, docker.Client())
	if err != nil {
		return nil, fmt.Errorf("failed to list apps status: %w", err)
	}

	// getAppsStatus iterates over a map, so sort the result to have a deterministic output.
	slices.SortFunc(appsStatus, func(a, b AppStatusInfo) int {
		return strings.Compare(a.AppPath.String(), b.AppPath.String())
	})

	return f.Map(appsStatus, func(s AppStatusInfo) AppInfo {
		info := AppInfo{Status: s.Status}

		// The app directory can be removed while the app is running: compute the ID from
		// the path in the Docker labels without requiring it to exist on disk.
		if id, err := idProvider.IDFromPathAllowMissing(s.AppPath); err != nil {
			slog.Warn("unable to get app id", slog.String("path", s.AppPath.String()), slog.String("error", err.Error()))
		} else {
			info.ID = id
			info.Example = id.IsExample()
			info.Release = id.IsRelease()
		}

		userApp, err := app.Load(s.AppPath)
		if err != nil {
			slog.Warn("unable to load app metadata", slog.String("path", s.AppPath.String()), slog.String("error", err.Error()))
			info.Name = s.AppPath.Base()
			return info
		}

		info.Name = userApp.Name
		info.Description = userApp.Descriptor.Description
		info.Icon = userApp.Descriptor.Icon
		return info
	}), nil
}

func AppDetails(
	ctx context.Context,
	docker command.Cli,
	userApp app.ArduinoApp,
	bricksIndex *bricksindex.BricksIndex,
	idProvider *appid.Provider,
	cfg config.Configuration,
) (AppDetailedInfo, error) {
	return appbook.AppDetails(ctx, userApp, func(ctx context.Context) (map[string]Status, error) {
		status := StatusStopped
		if app, err := getAppStatus(ctx, docker.Client(), userApp); err != nil {
			slog.Warn("unable to get app status", slog.String("error", err.Error()), slog.String("path", userApp.FullPath.String()))
		} else {
			status = app.Status
		}
		return map[string]Status{userApp.FullPath.String(): status}, nil
	}, bricksIndex, idProvider, cfg)
}

// CheckAppNotRunning returns ErrAppRunning if the app runs, starts or stops.
func CheckAppNotRunning(ctx context.Context, docker command.Cli, a app.ArduinoApp) error {
	status, err := getAppStatus(ctx, docker.Client(), a)
	if err != nil {
		return fmt.Errorf("failed to get app status: %w", err)
	}
	if status.Status == StatusRunning || status.Status == StatusStarting || status.Status == StatusStopping {
		return ErrAppRunning
	}
	return nil
}

func compileUploadSketch(
	ctx context.Context,
	verbose bool,
	platform platform.Platform,
	arduinoApp app.ArduinoApp,
	w io.Writer,
) error {
	sketchPath, ok := arduinoApp.GetSketchPath()
	if !ok {
		return fmt.Errorf("no sketch path found in the Arduino app")
	}
	// Make the cache dir for the compiled sketch
	buildPath := arduinoApp.SketchBuildPath()
	if err := buildPath.MkdirAll(); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	srv, inst, err := initializeArduinoCli(ctx, sketchPath, w)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = srv.Destroy(ctx, &rpc.DestroyRequest{Instance: inst})
	}()

	menuOptions, err := GetPlatformMenuOptions(ctx, platform)
	if err != nil {
		slog.Warn("failed to get platform menu options", slog.String("error", err.Error()))
	}
	fqbn := platform.FQBN
	if menuOptions.Has(WaitForApp) {
		fqbn += ":" + WaitForApp.String()
	}
	useLegacyFlashToRam := !menuOptions.Has(WaitForApp) && platform.SupportFlashToRam()

	// Compile the sketch
	if err := compileSketch(
		ctx, srv, inst,
		sketchPath, buildPath,
		platform, fqbn,
		verbose, w,
	); err != nil {
		return err
	}

	// Upload the sketch
	if err := uploadSketch(
		ctx, srv, inst,
		sketchPath, buildPath,
		useLegacyFlashToRam,
		fqbn, w,
	); err != nil {
		return err
	}

	return nil
}

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
