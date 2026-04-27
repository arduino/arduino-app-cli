// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"os/user"
	"slices"
	"strings"
	"sync"

	"github.com/arduino/arduino-cli/commands"
	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/arduino/go-paths-helper"
	"github.com/docker/cli/cli/command"
	"github.com/goccy/go-yaml"
	"github.com/gosimple/slug"
	"github.com/sirupsen/logrus"
	"go.bug.st/f"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/fatomic"
	"github.com/arduino/arduino-app-cli/internal/helpers"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	appgenerator "github.com/arduino/arduino-app-cli/internal/orchestrator/app/generator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/peripherals"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-app-cli/internal/store"
)

var (
	ErrAppAlreadyExists = fmt.Errorf("app already exists")
	ErrAppDoesntExists  = fmt.Errorf("app doesn't exist")
	ErrAppNotFound      = fmt.Errorf("app not found")
	ErrBadRequest       = fmt.Errorf("bad request")
)

const (
	DefaultDockerStopTimeoutSeconds = 5
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
	staticStore *store.StaticStore,
	platform platform.Platform,
	verbose bool,
	cb func(StreamMessage),
) error {
	bricksIndex = bricksIndex.WithAppBricks(appToStart.LocalBricks)

	if err := checkBricks(appToStart.Descriptor, bricksIndex, modelsIndex); err != nil {
		return err
	}

	devices, err := peripherals.Detect()
	if err != nil {
		return err
	}

	if err := checkRequiredDevices(bricksIndex, appToStart.Descriptor.Bricks, devices); err != nil {
		return err
	}

	if running, err := getRunningApp(ctx, docker.Client()); err != nil {
		return err
	} else if running != nil {
		return fmt.Errorf("app %q is running", running.Name)
	} else {
		cb(StreamMessage{data: fmt.Sprintf("Starting app %q", appToStart.Name)})
	}

	if err := setLedsToUserControlledMode(platform); err != nil {
		slog.Debug("unable to set status leds", slog.String("error", err.Error()))
	}

	sketchCallbackWriter := NewCallbackWriter(func(line string) {
		cb(StreamMessage{data: line})
	})

	cb(StreamMessage{progress: &Progress{Name: "preparing", Progress: 0.0}})

	if _, ok := appToStart.GetSketchPath(); ok {
		cb(StreamMessage{progress: &Progress{Name: "sketch compiling and uploading", Progress: 0.0}})

		if ok, err := migrateRemoveRouterBridgeIfNeeded(ctx, platform, appToStart); err != nil {
			cb(StreamMessage{data: "Failed to apply app migration for platform arduino:zephyr >0.54.1. Error: " + err.Error()})
		} else if ok {
			cb(StreamMessage{data: "Applied app migration for platform arduino:zephyr >0.54.1. Arduino_RouterBridge is now part of the platform and shouldn't be explicitly specified"})
		}

		if err := compileUploadSketch(ctx, verbose, platform, appToStart, sketchCallbackWriter); err != nil {
			return err
		}

		cb(StreamMessage{progress: &Progress{Name: "sketch updated", Progress: 10.0}})
	}

	if appToStart.MainPythonFile != nil {
		envs := getAppEnvironmentVariables(appToStart, bricksIndex, modelsIndex)

		cb(StreamMessage{data: "python provisioning"})
		provisionStartProgress := float32(0.0)
		if _, ok := appToStart.GetSketchPath(); ok {
			provisionStartProgress = 10.0
		}

		cb(StreamMessage{progress: &Progress{Name: "python provisioning", Progress: provisionStartProgress}})

		if err := provisioner.App(ctx, bricksIndex, servicesIndex, &appToStart, cfg, envs, platform, devices); err != nil {
			return err
		}

		cb(StreamMessage{data: "python downloading"})

		// Launch the docker compose command to start the app
		commands := []string{}
		commands = append(commands, "docker", "compose", "-f", appToStart.AppComposeFilePath().String())
		if overrideComposeFile := appToStart.AppComposeOverrideFilePath(); overrideComposeFile.Exist() {
			commands = append(commands, "-f", overrideComposeFile.String())
		}
		commands = append(commands, "up", "-d", "--remove-orphans", "--pull", "missing")

		dockerParser := NewDockerProgressParser(200)

		var customError error
		callbackDockerWriter := NewCallbackWriter(func(line string) {
			// docker compose sometimes returns errors as info lines, we try to parse them here and return a proper error
			if e := GetCustomErrorFomDockerEvent(line); e != nil {
				customError = e
			}
			if percentage, ok := dockerParser.Parse(line); ok {
				// assumption: docker pull progress goes from 0 to 80% of the total app start progress
				totalProgress := 20.0 + (percentage/100.0)*80.0
				cb(StreamMessage{progress: &Progress{Name: "python starting", Progress: float32(totalProgress)}})
				return
			}
			cb(StreamMessage{data: line})
		})

		slog.Debug("starting app", slog.String("command", strings.Join(commands, " ")), slog.Any("envs", envs))
		process, err := paths.NewProcess(envs.AsList(), commands...)
		if err != nil {
			return err
		}
		process.RedirectStderrTo(callbackDockerWriter)
		process.RedirectStdoutTo(callbackDockerWriter)
		if err := process.RunWithinContext(ctx); err != nil {
			// custom error could have been set while reading the output. Not detected by the process exit code
			if customError != nil {
				return customError
			}
			return err
		}
	}
	cb(StreamMessage{progress: &Progress{Name: "", Progress: 100.0}})
	return nil
}

// getAppEnvironmentVariables returns the environment variables for the app by merging variables and config in the following order:
// - brick default variables (variables defined in the brick definition)
// - model configuration variables (variables defined in the model configuration)
// - brick instance variables (variables defined in the app.yaml for the brick instance)
// In addition, it adds some useful environment variables like APP_HOME and HOST_IP.
func getAppEnvironmentVariables(app app.ArduinoApp, brickIndex *bricksindex.BricksIndex, modelsIndex *modelsindex.ModelsIndex) helpers.EnvVars {
	envs := make(helpers.EnvVars)

	for _, brick := range app.Descriptor.Bricks {
		if brickDef, found := brickIndex.WithAppBricks(app.LocalBricks).FindBrickByID(brick.ID); found {
			maps.Insert(envs, brickDef.GetDefaultVariables())
		}

		if m, found := modelsIndex.GetModelByID(brick.Model); found {
			for _, b := range m.Bricks {
				maps.Insert(envs, maps.All(b.ModelConfiguration))
			}
		}

		slog.Debug("adding Brick", slog.String("brickID", brick.ID), slog.String("model", brick.Model), slog.Any("variables", brick.Variables))
		maps.Insert(envs, maps.All(brick.Variables))
	}

	// Add the APP_HOME directory to the environment variables
	envs["APP_HOME"] = app.FullPath.String()

	// Pre-select default camera device if available. This can be overridden by the app environment variables (or in future by applab)
	// This is required because there are some video devices for HW acceleration that are auto registered in /dev but are not real cameras.
	if videoDevices := peripherals.GetVideoDevices(); len(videoDevices) > 0 {
		// VIDEO_DEVICE will be the first device in /dev/v4l/by-id
		envs["VIDEO_DEVICE"] = videoDevices[0]
	}

	if mediaCarriers := peripherals.GetMediaCarriers(); len(mediaCarriers) > 0 {
		envs["CONNECTED_CARRIERS"] = strings.Join(mediaCarriers, ",")
	}
	if hostIP, err := helpers.GetHostIP(); err == nil {
		envs["HOST_IP"] = hostIP
	} else {
		slog.Warn("unable to get host IP", slog.String("error", err.Error()))
	}

	slog.Debug("Current environment variables", slog.Any("envs", envs))

	return envs
}

func stopAppWithCmd(ctx context.Context, docker command.Cli, platform platform.Platform, app app.ArduinoApp, cmd string, cb func(StreamMessage)) error {
	switch cmd {
	case "stop":
		cb(StreamMessage{data: fmt.Sprintf("Stopping app %q", app.Name)})
	case "down":
		cb(StreamMessage{data: fmt.Sprintf("Destroying  app %q", app.Name)})
	}

	if err := restoreLedsState(platform); err != nil {
		slog.Debug("unable to set status leds", slog.String("error", err.Error()))
	}

	callbackWriter := NewCallbackWriter(func(line string) {
		cb(StreamMessage{data: line})
	})

	if _, ok := app.GetSketchPath(); ok {
		// Before stopping the microcontroller we want to make sure that the app was running.
		running, err := getRunningApp(ctx, docker.Client())
		if err != nil {
			return err
		}
		if running != nil && running.FullPath.String() == app.FullPath.String() {
			cb(StreamMessage{data: "Stopping microcontroller..."})
			if err := platform.GetMicro().Disable(); err != nil {
				return err
				// XXX: if we fail to stop the sketch, do we want to continue to stop the app anyway?
				//      maybe we can just log the error and continue
			}
		}
	}

	if app.MainPythonFile != nil {
		mainCompose := app.AppComposeFilePath()
		// In case the app was never started
		if mainCompose.Exist() {
			args := []string{
				"docker",
				"compose",
				"-f", mainCompose.String(),
				cmd,
				fmt.Sprintf("--timeout=%d", DefaultDockerStopTimeoutSeconds),
			}
			if cmd == "down" {
				args = append(args, "--volumes", "--remove-orphans")
			}

			process, err := paths.NewProcess(nil, args...)
			if err != nil {
				return err
			}

			process.RedirectStderrTo(callbackWriter)
			process.RedirectStdoutTo(callbackWriter)
			if err := process.RunWithinContext(ctx); err != nil {
				return err
			}
		}
	}
	cb(StreamMessage{progress: &Progress{Name: "", Progress: 100.0}})
	return nil
}

func StopApp(ctx context.Context, dockerClient command.Cli, platform platform.Platform, app app.ArduinoApp, cb func(StreamMessage)) error {
	return stopAppWithCmd(ctx, dockerClient, platform, app, "stop", cb)
}

func StopAndDestroyApp(ctx context.Context, dockerClient command.Cli, platform platform.Platform, app app.ArduinoApp, cb func(StreamMessage)) error {
	if err := stopAppWithCmd(ctx, dockerClient, platform, app, "down", cb); err != nil {
		return err
	}
	if err := cleanAppCacheFiles(app, cb); err != nil {
		return err
	}
	return nil
}

func cleanAppCacheFiles(app app.ArduinoApp, cb func(StreamMessage)) error {
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
	staticStore *store.StaticStore,
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

		if err := StopApp(ctx, docker, platform, *runningApp, cb); err != nil {
			return err
		}
	}

	return StartApp(ctx, docker, provisioner, modelsIndex, bricksIndex, servicesIndex, appToStart, cfg, staticStore, platform, verbose, cb)
}

func StartDefaultApp(
	ctx context.Context,
	docker command.Cli,
	provisioner *Provision,
	modelsIndex *modelsindex.ModelsIndex,
	bricksIndex *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
	idProvider *app.IDProvider,
	cfg config.Configuration,
	staticStore *store.StaticStore,
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
	if err := StartApp(ctx, docker, provisioner, modelsIndex, bricksIndex, servicesIndex, *app, cfg, staticStore, platform, false, func(sm StreamMessage) {}); err != nil {
		return fmt.Errorf("failed to start app: %w", err)
	}

	return nil
}

type ListAppResult struct {
	Apps       []AppInfo       `json:"apps"`
	BrokenApps []BrokenAppInfo `json:"broken_apps"`
}

type AppInfo struct {
	ID          app.ID `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Status      Status `json:"status,omitempty"`
	Example     bool   `json:"example"`
	Default     bool   `json:"default"`
}

type BrokenAppInfo struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

type ListAppRequest struct {
	ShowExamples    bool
	ShowOnlyDefault bool
	ShowApps        bool
	StatusFilter    Status

	// IncludeNonStandardLocationApps will include apps that are not in the standard apps directory.
	// We will search by looking for docker container metadata, and add the app not present in the
	// standard apps directory in the result list.
	IncludeNonStandardLocationApps bool
}

func ListApps(
	ctx context.Context,
	docker command.Cli,
	req ListAppRequest,
	idProvider *app.IDProvider,
	bricksIndex *bricksindex.BricksIndex,
	cfg config.Configuration,
) (ListAppResult, error) {
	// Get the default app to mark it in the list
	defaultApp, err := GetDefaultApp(cfg)
	if err != nil {
		slog.Warn("unable to get default app", slog.String("error", err.Error()))
	}

	// Get the status of all apps (it will return only the apps that have been started at least once)
	appsStatus, err := getAppsStatus(ctx, docker.Client())
	if err != nil {
		slog.Error("unable to get running app", slog.String("error", err.Error()))
	}

	// Retrieve all apps from the filesystem
	var pathsToExplore paths.PathList
	var appPaths paths.PathList
	if req.ShowExamples || req.ShowOnlyDefault {
		pathsToExplore.Add(cfg.ExamplesDir())
	}
	if req.ShowApps || req.ShowOnlyDefault {
		pathsToExplore.Add(cfg.AppsDir())
		// and optionally add apps that are on different paths
		if req.IncludeNonStandardLocationApps {
			for _, appStatus := range appsStatus {
				appPaths.AddIfMissing(appStatus.AppPath)
			}
		}
	}
	for _, p := range pathsToExplore {
		res, err := app.FindAppsInFolder(p)
		if err != nil {
			slog.Error("unable to list apps", slog.String("error", err.Error()))
			return ListAppResult{}, err
		}
		appPaths.AddAllMissing(res)
	}

	// Compose the result
	result := ListAppResult{Apps: []AppInfo{}, BrokenApps: []BrokenAppInfo{}}
	for _, file := range appPaths {
		app, err := app.Load(file)
		if err != nil {
			result.BrokenApps = append(result.BrokenApps, BrokenAppInfo{
				Name:  file.Base(),
				Error: fmt.Sprintf("unable to parse the app.yaml: %s", err.Error()),
			})
			continue
		}

		// Apply default-apps-only filter if requested
		isDefault := defaultApp != nil && defaultApp.FullPath.EqualsTo(app.FullPath)
		if req.ShowOnlyDefault && !isDefault {
			continue
		}

		// Retrieve the app status if available
		status := StatusUninitialized
		if idx := slices.IndexFunc(appsStatus, func(a AppStatusInfo) bool {
			return a.AppPath.EqualsTo(app.FullPath)
		}); idx != -1 {
			status = appsStatus[idx].Status
		}

		// Apply status filter if requested
		if req.StatusFilter != "" && req.StatusFilter != status {
			continue
		}

		// Get the app ID
		id, err := idProvider.IDFromPath(app.FullPath)
		if err != nil {
			return ListAppResult{}, fmt.Errorf("failed to get app ID from path %s: %w", file.String(), err)
		}

		// Filter example apps that use bricks incompatible with the board
		if bricksIndex != nil && id.IsExample() && !exampleCompatibleWithBricksIndex(app, bricksIndex) {
			continue
		}

		result.Apps = append(result.Apps,
			AppInfo{
				ID:          id,
				Name:        app.Name,
				Description: app.Descriptor.Description,
				Icon:        app.Descriptor.Icon,
				Status:      status,
				Example:     id.IsExample(),
				Default:     isDefault,
			},
		)
	}

	return result, nil
}

// exampleCompatibleWithBricksIndex returns true if all built-in bricks referenced by the app
// are present in the given bricks index. Local bricks bundled with the app are always treated
// as compatible and are skipped in the check.
func exampleCompatibleWithBricksIndex(a app.ArduinoApp, idx *bricksindex.BricksIndex) bool {
	localBrickIDs := make(map[string]struct{}, len(a.LocalBricks))
	for _, lb := range a.LocalBricks {
		localBrickIDs[lb.ID] = struct{}{}
	}
	for _, brick := range a.Descriptor.Bricks {
		if _, isLocal := localBrickIDs[brick.ID]; isLocal {
			continue
		}
		if _, ok := idx.FindBrickByID(brick.ID); !ok {
			return false
		}
	}
	return true
}

type AppDetailedInfo struct {
	ID          app.ID             `json:"id" required:"true" `
	Name        string             `json:"name" required:"true"`
	Path        string             `json:"path"`
	Description string             `json:"description"`
	Icon        string             `json:"icon"`
	Status      Status             `json:"status" required:"true"`
	Example     bool               `json:"example"`
	Default     bool               `json:"default"`
	Bricks      []AppDetailedBrick `json:"bricks,omitempty"`
}

type AppDetailedBrick struct {
	ID           string `json:"id" required:"true"`
	Name         string `json:"name" required:"true"`
	Category     string `json:"category,omitempty"`
	RequireModel bool   `json:"require_model"`
}

func AppDetails(
	ctx context.Context,
	docker command.Cli,
	userApp app.ArduinoApp,
	bricksIndex *bricksindex.BricksIndex,
	idProvider *app.IDProvider,
	cfg config.Configuration,
) (AppDetailedInfo, error) {
	bricksIndex = bricksIndex.WithAppBricks(userApp.LocalBricks)
	var wg sync.WaitGroup
	wg.Add(2)
	var defaultAppPath string
	var status Status
	go func() {
		defer wg.Done()
		app, err := getAppStatus(ctx, docker.Client(), userApp)
		if err != nil {
			slog.Warn("unable to get app status", slog.String("error", err.Error()), slog.String("path", userApp.FullPath.String()))
			status = StatusStopped
		} else {
			status = app.Status
		}
	}()
	go func() {
		defer wg.Done()
		defaultApp, err := GetDefaultApp(cfg)
		if err != nil {
			slog.Warn("unable to get default app", slog.String("error", err.Error()))
			return
		}
		if defaultApp == nil {
			return
		}
		defaultAppPath = defaultApp.FullPath.String()

	}()
	wg.Wait()

	id, err := idProvider.IDFromPath(userApp.FullPath)
	if err != nil {
		return AppDetailedInfo{}, err
	}

	return AppDetailedInfo{
		ID:          id,
		Name:        userApp.Name,
		Path:        userApp.FullPath.String(),
		Description: userApp.Descriptor.Description,
		Icon:        userApp.Descriptor.Icon,
		Status:      status,
		Example:     id.IsExample(),
		Default:     defaultAppPath == userApp.FullPath.String(),
		Bricks: f.Map(userApp.Descriptor.Bricks, func(b app.Brick) AppDetailedBrick {
			res := AppDetailedBrick{ID: b.ID}
			bi, found := bricksIndex.FindBrickByID(b.ID)
			if !found {
				slog.Warn("brick not found in bricks index", slog.String("id", b.ID), slog.String("app", userApp.FullPath.String()))
				return res
			}
			res.Name = bi.Name
			res.Category = bi.Category
			res.RequireModel = bi.RequireModel
			return res
		}),
	}, nil
}

type CreateAppRequest struct {
	Name        string
	Icon        string
	Description string
	SkipSketch  bool
}

type CreateAppResponse struct {
	ID app.ID `json:"id"`
}

func CreateApp(
	ctx context.Context,
	req CreateAppRequest,
	idProvider *app.IDProvider,
	cfg config.Configuration,
) (CreateAppResponse, error) {
	if req.Name == "" {
		return CreateAppResponse{}, fmt.Errorf("app name cannot be empty")
	}

	basePath, appExists := findAppPathByName(req.Name, cfg)
	if appExists {
		return CreateAppResponse{}, ErrAppAlreadyExists
	}
	appName := req.Name
	newApp := app.AppDescriptor{
		Name:        appName,
		Description: req.Description,
		Ports:       []int{},
		Icon:        req.Icon, // TODO: not sure if icon will exists for bricks
	}
	if err := newApp.IsValid(); err != nil {
		return CreateAppResponse{}, fmt.Errorf("%w: %v", app.ErrInvalidApp, err)
	}

	if err := appgenerator.GenerateApp(basePath, newApp, req.SkipSketch); err != nil {
		return CreateAppResponse{}, fmt.Errorf("failed to create app: %w", err)
	}
	id, err := idProvider.IDFromPath(basePath)
	if err != nil {
		return CreateAppResponse{}, fmt.Errorf("failed to get app id: %w", err)
	}
	return CreateAppResponse{ID: id}, nil
}

type CloneAppRequest struct {
	FromID app.ID

	Name *string
	Icon *string
}

type CloneAppResponse struct {
	ID app.ID `json:"id"`
}

func CloneApp(
	ctx context.Context,
	req CloneAppRequest,
	idProvider *app.IDProvider,
	cfg config.Configuration,
) (response CloneAppResponse, cloneErr error) {
	originPath := req.FromID.ToPath()
	if !originPath.Exist() {
		return CloneAppResponse{}, ErrAppDoesntExists
	}
	if !originPath.Join("app.yaml").Exist() && !originPath.Join("app.yml").Exist() {
		return CloneAppResponse{}, app.ErrInvalidApp
	}

	var dstPath *paths.Path
	if req.Name != nil && *req.Name != "" {
		dstPath = cfg.AppsDir().Join(slug.Make(*req.Name))
		if dstPath.Exist() {
			return CloneAppResponse{}, ErrAppAlreadyExists
		}
	} else {
		for i := range 100 { // In case of name collision, we try up to 100 times.
			dstName := fmt.Sprintf("%s-copy%d", originPath.Base(), i)
			dstPath = cfg.AppsDir().Join(dstName)
			if !dstPath.Exist() {
				break
			}
		}
	}
	if err := dstPath.MkdirAll(); err != nil {
		return CloneAppResponse{}, fmt.Errorf("failed to create app directory: %w", err)
	}

	// In case something during the clone operation fails we remove the dst path
	defer func() {
		if cloneErr != nil {
			_ = dstPath.RemoveAll()
		}
	}()

	list, err := originPath.ReadDir(paths.FilterOutNames(".cache", "data"))
	if err != nil {
		return CloneAppResponse{}, fmt.Errorf("failed to read app directory: %w", err)
	}
	for _, file := range list {
		if file.IsDir() {
			if err := file.CopyDirTo(dstPath.Join(file.Base())); err != nil {
				return CloneAppResponse{}, fmt.Errorf("failed to copy directory: %w", err)
			}
		} else {
			if err := file.CopyTo(dstPath.Join(file.Base())); err != nil {
				return CloneAppResponse{}, fmt.Errorf("failed to copy file: %w", err)
			}
		}
	}

	if (req.Name != nil && *req.Name != "") || (req.Icon != nil && *req.Icon != "") {
		var appYamlPath *paths.Path
		if dstPath.Join("app.yaml").Exist() {
			appYamlPath = dstPath.Join("app.yaml")
		} else {
			appYamlPath = dstPath.Join("app.yml")
		}
		descriptor, err := app.ParseDescriptorFile(appYamlPath)
		if err != nil {
			return CloneAppResponse{}, fmt.Errorf("failed to parse app.yaml file: %w", err)
		}
		if req.Name != nil && *req.Name != "" {
			descriptor.Name = *req.Name
		}
		if req.Icon != nil && *req.Icon != "" {
			descriptor.Icon = *req.Icon
		}

		// TODO: implement MarshalYaml directly in the descriptor.
		newDescriptor, err := yaml.Marshal(descriptor)
		if err != nil {
			// TODO: should we consider this a fatal error, or we prefer to silently ignore the error?
			// Worst case, the optional fields will be the same as the source app.
			return CloneAppResponse{}, fmt.Errorf("failed to marshal app.yaml file: %w", err)
		}
		if err := appYamlPath.WriteFile(newDescriptor); err != nil {
			return CloneAppResponse{}, fmt.Errorf("failed to write app.yaml file: %w", err)
		}
	}

	id, err := idProvider.IDFromPath(dstPath)
	if err != nil {
		return CloneAppResponse{}, fmt.Errorf("failed to get app id: %w", err)
	}
	return CloneAppResponse{ID: id}, nil
}

func DeleteApp(ctx context.Context, dockerClient command.Cli, platform platform.Platform, app app.ArduinoApp) error {
	// We try to remove docker related resources at best effort
	_ = StopAndDestroyApp(ctx, dockerClient, platform, app, func(StreamMessage) {})
	// TODO: Shall we report stop error?

	return app.FullPath.RemoveAll()
}

const defaultAppFileName = "default.app"

func SetDefaultApp(app *app.ArduinoApp, cfg config.Configuration) error {
	defaultAppPath := cfg.DataDir().Join(defaultAppFileName)

	// Remove the default app file if the app is nil.
	if app == nil {
		err := defaultAppPath.Remove()
		if err != nil {
			slog.Warn("failed to remove default app file", slog.String("path", defaultAppPath.String()), slog.String("error", err.Error()))
		}
		return nil
	}

	return fatomic.WriteFile(defaultAppPath.String(), []byte(app.FullPath.String()), os.FileMode(0644))
}

func GetDefaultApp(cfg config.Configuration) (*app.ArduinoApp, error) {
	defaultAppFilePath := cfg.DataDir().Join(defaultAppFileName)
	if !defaultAppFilePath.Exist() {
		return nil, nil
	}

	defaultAppPath, err := defaultAppFilePath.ReadFile()
	if err != nil {
		return nil, err
	}
	defaultAppPath = bytes.TrimSpace(defaultAppPath)
	if len(defaultAppPath) == 0 {
		// If the file is empty, we remove it
		slog.Warn("default app file is empty", slog.String("path", string(defaultAppPath)))
		_ = defaultAppFilePath.Remove()
		return nil, nil
	}

	app, err := app.Load(paths.New(string(defaultAppPath)))
	if err != nil {
		// If the app is not valid, we remove the file
		slog.Warn("default app is not valid", slog.String("path", string(defaultAppPath)), slog.String("error", err.Error()))
		_ = defaultAppFilePath.Remove()
		return nil, err
	}

	return &app, nil
}

type AppEditRequest struct {
	Name        *string
	Icon        *string
	Description *string
	Default     *bool
}

func EditApp(
	req AppEditRequest,
	editApp *app.ArduinoApp,
	cfg config.Configuration,
) (editErr error) {
	if req.Default != nil {
		if err := editAppDefaults(editApp, *req.Default, cfg); err != nil {
			return fmt.Errorf("failed to edit app defaults: %w", err)
		}
	}

	if req.Name != nil {
		editApp.Descriptor.Name = *req.Name
		newPath := editApp.FullPath.Parent().Join(slug.Make(*req.Name))
		if newPath.Exist() {
			return ErrAppAlreadyExists
		}
		if err := editApp.FullPath.Rename(newPath); err != nil {
			editErr = fmt.Errorf("failed to rename app path: %w", err)
			return editErr
		}
		editApp.FullPath = newPath
		editApp.Name = editApp.Descriptor.Name
	}

	if req.Icon != nil {
		editApp.Descriptor.Icon = *req.Icon
	}
	if req.Description != nil {
		editApp.Descriptor.Description = *req.Description
	}

	if err := editApp.Descriptor.IsValid(); err != nil {
		return fmt.Errorf("%w: %w", app.ErrInvalidApp, err)
	}
	err := editApp.Save()
	if err != nil {
		return fmt.Errorf("failed to save app: %w", err)
	}
	return nil
}

func editAppDefaults(userApp *app.ArduinoApp, isDefault bool, cfg config.Configuration) error {
	if isDefault {
		if err := SetDefaultApp(userApp, cfg); err != nil {
			return fmt.Errorf("failed to set default app: %w", err)
		}
		return nil
	}

	defaultApp, err := GetDefaultApp(cfg)
	if err != nil {
		return fmt.Errorf("failed to get default app: %w", err)
	}

	// No default app set, nothing to unset.
	if defaultApp == nil {
		return nil
	}

	// Unset only if the current default is the same as the app being edited.
	if defaultApp.FullPath.String() == userApp.FullPath.String() {
		if err := SetDefaultApp(nil, cfg); err != nil {
			return fmt.Errorf("failed to unset default app: %w", err)
		}
	}
	return nil
}

func getCurrentUser() string {
	userInfo := f.Must(user.Current())
	uid := userInfo.Uid
	gid := userInfo.Gid

	// If exist use arduino group to avoid permission issue on files /var/lib/arduino-app-cli in.
	if gInfo, err := user.LookupGroup("arduino"); err == nil {
		gid = gInfo.Gid
	}

	return uid + ":" + gid
}

// addLedControl adds bindings for led control if the paths exist.
func addLedControl(platform platform.Platform, volumes []volume) []volume {
	for _, led := range platform.Linux.BoardLeds {

		if led.Exist() {
			volumes = append(volumes, volume{
				Type:   "bind",
				Source: led.String(),
				Target: led.String(),
			})
		}
	}

	return volumes
}

func compileUploadSketch(
	ctx context.Context,
	verbose bool,
	platform platform.Platform,
	arduinoApp app.ArduinoApp,
	w io.Writer,
) error {
	logrus.SetLevel(logrus.ErrorLevel) // Reduce the log level of arduino-cli
	srv := commands.NewArduinoCoreServer()
	if err := SetArduinoCliConfig(ctx, srv); err != nil {
		return err
	}

	var inst *rpc.Instance
	if resp, err := srv.Create(ctx, &rpc.CreateRequest{}); err != nil {
		return err
	} else {
		inst = resp.GetInstance()
	}
	defer func() {
		_, _ = srv.Destroy(ctx, &rpc.DestroyRequest{Instance: inst})
	}()

	sketchPath, ok := arduinoApp.GetSketchPath()
	if !ok {
		return fmt.Errorf("no sketch path found in the Arduino app")
	}
	sketchResp, err := srv.LoadSketch(ctx, &rpc.LoadSketchRequest{SketchPath: sketchPath.String()})
	if err != nil {
		return err
	}
	sketch := sketchResp.GetSketch()
	profile := sketch.GetDefaultProfile().GetName()
	if profile == "" {
		return fmt.Errorf("sketch %q has no default profile", sketchPath)
	}
	initReq := &rpc.InitRequest{
		Instance:   inst,
		SketchPath: sketchPath.String(),
		Profile:    profile,
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
		return err
	}

	menuOptions, err := GetPlatformMenuOptions(ctx, platform)
	if err != nil {
		slog.Warn("failed to get platform menu options", slog.String("error", err.Error()))
	}

	fqbn := platform.FQBN
	if menuOptions.Has(WaitForApp) {
		fqbn += ":" + WaitForApp.String()
	}

	slog.Debug("compile and upload sketch", slog.String("fqbn", fqbn), slog.Any("menuOptions", menuOptions))

	// build the sketch
	buildPath := arduinoApp.SketchBuildPath()
	if buildPath.NotExist() {
		if err := buildPath.MkdirAll(); err != nil {
			return fmt.Errorf("failed to create build directory: %w", err)
		}
	}

	server, getCompileResult := commands.CompilerServerToStreams(ctx, w, w, nil)
	compileReq := rpc.CompileRequest{
		Instance:   inst,
		Fqbn:       fqbn,
		SketchPath: sketchPath.String(),
		BuildPath:  buildPath.String(),
		Jobs:       platform.CompileJobs,
		Verbose:    verbose,
	}
	err = srv.Compile(&compileReq, server)
	if err != nil {
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

	// Support the legacy ram upload option if there isn't the new wait_linux_boot option.
	if !menuOptions.Has(WaitForApp) && platform.SupportFlashToRam() {
		if err := legacyUploadSketchInRam(ctx, w, srv, inst, platform, sketchPath.String(), buildPath.String()); err != nil {
			slog.Warn("failed to upload in ram mode, trying to configure the board in ram mode, and retry", slog.String("error", err.Error()))
			if err := configureMicroInRamMode(ctx, w, srv, inst, platform); err != nil {
				return err
			}
			return legacyUploadSketchInRam(ctx, w, srv, inst, platform, sketchPath.String(), buildPath.String())
		}
		return nil
	}

	stream, _ := commands.UploadToServerStreams(ctx, w, w)
	return srv.Upload(&rpc.UploadRequest{
		Instance:   inst,
		Fqbn:       platform.FQBN,
		SketchPath: sketchPath.String(),
		ImportDir:  buildPath.String(),
	}, stream)
}

// migrateRemoveRouterBridgeIfNeeded removes the Arduino_RouterBridge library from the sketch profile to allow automatic update of the library.
// This is needed by the platform 0.55 will need a new Arduino_RouterBridge library to allow Serial output redirection to Monitor.
// The migration is applied only if the platform in the profile doesn't specify a version.
func migrateRemoveRouterBridgeIfNeeded(ctx context.Context, platform platform.Platform, app app.ArduinoApp) (bool, error) {
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
		libs, err := ListSketchLibraries(ctx, app)
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

type ConfigResponse struct {
	Directories ConfigDirectories `json:"directories"`
}

type ConfigDirectories struct {
	Data     string `json:"data"`
	Apps     string `json:"apps"`
	Examples string `json:"examples"`
}

func GetOrchestratorConfig(cfg config.Configuration) ConfigResponse {
	return ConfigResponse{
		Directories: ConfigDirectories{
			Data:     cfg.DataDir().String(),
			Apps:     cfg.AppsDir().String(),
			Examples: cfg.ExamplesDir().String(),
		},
	}
}
