// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/arduino/arduino-cli/commands"
	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	dockerClient "github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

var ErrDockerOutOfSpace = errors.New("not enough disk space to pull the docker image")

const ExitCodeDockerOutOfSpace = 80

type InitProgress struct {
	Label string
	Curr  int64
	Total int64
}

type InitEventType int

const (
	InitLogEvent InitEventType = iota
	InitProgressEvent
)

type InitEventSource string

const (
	InitSourceDocker  InitEventSource = "docker"
	InitSourceArduino InitEventSource = "arduino"
	InitSourceDeb     InitEventSource = "deb"
)

type InitEvent struct {
	Type     InitEventType
	Source   InitEventSource
	Message  string
	Progress InitProgress
}

// InitEventCallback is the sole output sink for SystemInit.
type InitEventCallback func(event InitEvent)

type SystemInitOptions struct {
	OnlyDockerImages    bool
	OnlyPlatformAndLibs bool
}

func (o SystemInitOptions) Validate() error {
	if o.OnlyDockerImages && o.OnlyPlatformAndLibs {
		return errors.New("only one of OnlyDockerImages and OnlyPlatformAndLibs can be true")
	}
	return nil
}

// SystemInit pulls all the docker images needed for the current version of the software to run and the
// sketch libraries used in the example apps. Can be used to pre-install docker images/libraries on an
// empty system, or to update all the docker images/libraries that need it.
func SystemInit(ctx context.Context, cfg config.Configuration, platform platform.Platform, bricksindex *bricksindex.BricksIndex, servicesindex *servicesindex.ServicesIndex, docker *command.DockerCli, modelsIndex *modelsindex.ModelsIndex, options SystemInitOptions, eventCB InitEventCallback) error {
	if eventCB == nil {
		eventCB = func(InitEvent) {}
	}
	if err := options.Validate(); err != nil {
		return err
	}

	var downloadPlatformAndLibs, downloadDockerImages bool
	switch {
	case options.OnlyPlatformAndLibs:
		downloadPlatformAndLibs = true
	case options.OnlyDockerImages:
		downloadDockerImages = true
	default:
		downloadPlatformAndLibs = true
		downloadDockerImages = true
	}

	if err := installPlatformPackage(ctx, platform, eventCB); err != nil {
		slog.Error("failed to install platform package", "error", err)
	}

	if downloadPlatformAndLibs {
		eventCB(InitEvent{Type: InitLogEvent, Source: InitSourceArduino, Message: "Downloading libs and platforms used in examples ..."})
		if err := downloadLibsAndPlatformsUsedInExamples(ctx, cfg, platform, eventCB); err != nil {
			return fmt.Errorf("failed to download libs and platforms used in examples: %w", err)
		}
	}

	if downloadDockerImages {
		if err := downloadSupportedImages(ctx, cfg, bricksindex, servicesindex, modelsIndex, docker, eventCB); err != nil {
			return fmt.Errorf("failed to download container images used in examples: %w", err)
		}
	}

	return nil
}

func downloadSupportedImages(ctx context.Context, cfg config.Configuration, brickindex *bricksindex.BricksIndex, servicesindex *servicesindex.ServicesIndex, modelsIndex *modelsindex.ModelsIndex, docker *command.DockerCli, eventCB InitEventCallback) error {
	eventCB(InitEvent{Type: InitLogEvent, Source: InitSourceDocker, Message: "Pulling the latest docker images ..."})
	imagesToPreinstall := []string{cfg.PythonImage}
	brickImages, err := getAllSupportedBrickImages(brickindex, servicesindex)
	if err != nil {
		return err
	}

	handlerImages := modelsIndex.Handlers.GetDockerImages()

	imagesToPreinstall = append(imagesToPreinstall, brickImages...)
	imagesToPreinstall = append(imagesToPreinstall, handlerImages...)

	pulledImages, err := listImagesAlreadyPulled(ctx, docker.Client())
	if err != nil {
		return err
	}

	// Filter out container images that are already pulled
	imagesToPreinstall = slices.DeleteFunc(imagesToPreinstall, func(v string) bool {
		return slices.Contains(pulledImages, v)
	})

	imagesToPull := make([]string, 0, len(imagesToPreinstall))
	allLayers := make([]dockerImageLayer, 0, len(imagesToPreinstall))
	for _, image := range imagesToPreinstall {
		previousExistingImage := GetHighestVersion(image, pulledImages)
		layers, err := missingLayers(previousExistingImage, image)
		if err != nil {
			slog.Warn("Unable to get the new image layers size", "image", image, "error", err)
		}
		var bytes int64
		for _, l := range layers {
			bytes += l.Size
		}
		imagesToPull = append(imagesToPull, image)
		allLayers = append(allLayers, layers...)
		slog.Info("docker image to download", "image", image, "bytes", bytes)
	}
	totalBytes := sumUniqueLayers(allLayers)
	slog.Info("total docker images download size", "bytes", totalBytes)

	// Check that there is enough disk space for all the layers to download.
	// Shared layers are counted once, so this is the real download size.
	freeSpace, err := GetDockerFreeSpace()
	if err != nil {
		return err
	}
	if uint64(float64(totalBytes)*2.5) > freeSpace {
		return ErrDockerOutOfSpace
	}

	layerProgress := map[string]int64{}
	var lastLabel string
	for i, image := range imagesToPull {
		eventCB(InitEvent{Type: InitLogEvent, Source: InitSourceDocker, Message: fmt.Sprintf("Pulling container image %s ...", image)})
		// The percentage stays global (across all images); the label tells the
		// user which image is currently being pulled.
		lastLabel = fmt.Sprintf("Pulling image %d/%d (%s)", i+1, len(imagesToPull), imageName(image))
		if err := pullImage(ctx, docker.Client(), image, layerProgress, totalBytes, lastLabel, eventCB); err != nil {
			return fmt.Errorf("failed to pull image %s: %w", image, err)
		}
	}

	if totalBytes > 0 && len(imagesToPull) > 0 {
		eventCB(InitEvent{Type: InitProgressEvent, Source: InitSourceDocker, Progress: InitProgress{Label: lastLabel, Curr: totalBytes, Total: totalBytes}})
	}

	return nil
}

const minDelay = 1 * time.Second
const maxDelay = 10 * time.Second

// updateLayerProgress records the bytes downloaded so far for a single layer
func updateLayerProgress(layerProgress map[string]int64, status, id string, current int64) int64 {
	if status == "Downloading" && id != "" {
		layerProgress[id] = current
	}

	var downloaded int64
	for _, c := range layerProgress {
		downloaded += c
	}
	return downloaded
}

func pullImage(ctx context.Context, docker dockerClient.APIClient, imageName string, layerProgress map[string]int64, totalBytes int64, progressLabel string, eventCB InitEventCallback) error {
	delay := minDelay
	var out io.ReadCloser
	var allErr error
	var lastErr error
	for range 10 { // 1s, 2s, 4s, 8s, 10s, 10s, 10s, 10s, 10s, 10s
		out, lastErr = docker.ImagePull(ctx, imageName, image.PullOptions{})
		if lastErr == nil {
			break // Success
		}
		allErr = errors.Join(allErr, lastErr)

		if !isTemporaryDockerError(lastErr) {
			return allErr // Non-retryable error
		}

		eventCB(InitEvent{Type: InitLogEvent, Source: InitSourceDocker, Message: fmt.Sprintf("received 'toomanyrequests' error from Docker registry, retrying in %s ...", delay)})

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		delay = min(delay*2, maxDelay)
	}
	if lastErr != nil {
		return fmt.Errorf("failed to pull image %s after multiple attempts: %w", imageName, allErr)
	}
	defer out.Close()

	scanner := bufio.NewScanner(out)
	for scanner.Scan() {
		type Payload struct {
			Status         string `json:"status"`
			Progress       string `json:"progress"`
			ID             string `json:"id"`
			ProgressDetail struct {
				Current int64 `json:"current"`
				Total   int64 `json:"total"`
			} `json:"progressDetail"`
		}

		var payload Payload
		if err := json.Unmarshal(scanner.Bytes(), &payload); err == nil {
			// Accumulate the downloaded bytes across all layers/images and report
			// the global download progress.
			downloaded := updateLayerProgress(layerProgress, payload.Status, payload.ID, payload.ProgressDetail.Current)
			if totalBytes > 0 {
				if downloaded > totalBytes {
					downloaded = totalBytes
				}
				eventCB(InitEvent{Type: InitProgressEvent, Source: InitSourceDocker, Progress: InitProgress{Label: progressLabel, Curr: downloaded, Total: totalBytes}})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}
func isTemporaryDockerError(err error) bool {
	errorString := err.Error()
	transientSubstrings := []string{
		"toomanyrequests",
		"Client.Timeout exceeded",
		"request canceled while waiting for connection",
	}

	for _, sub := range transientSubstrings {
		if strings.Contains(errorString, sub) {
			return true
		}
	}
	return false
}

// List of prefixes used to identify current or past Arduino images. Used both during 'system init' and during cleanup.
var imagePrefixes = []string{
	"ghcr.io/bcmi-labs/",
	"public.ecr.aws/arduino/",
	"ghcr.io/arduino/",
	"influxdb",
	"artifacts.codelinaro.org/iot-solutions-microservices/",
}

// Lists all the local docker images that could have been, or are downloaded by Arduino.
// This is used both to avoid pulling already existing images and cleaning up unused old Arduino images.
func listImagesAlreadyPulled(ctx context.Context, docker dockerClient.APIClient) ([]string, error) {
	images, err := docker.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(images))
	for _, image := range images {
		for _, tag := range image.RepoTags {
			if slices.ContainsFunc(imagePrefixes, func(p string) bool {
				return strings.HasPrefix(tag, p)
			}) {
				result = append(result, tag)
			}
		}
	}

	return result, nil
}

func getAllSupportedBrickImages(bricksIndex *bricksindex.BricksIndex, servicesIndex *servicesindex.ServicesIndex) ([]string, error) {
	var result []string
	for _, brick := range bricksIndex.ListBricks() {
		if composeFile, ok := brick.GetComposeFile(); ok {
			images, err := extractImagesFromCompose(composeFile)
			if err != nil {
				return nil, err
			}
			result = append(result, images...)
		}

		for _, r := range brick.RequiresServices {
			service, ok := servicesIndex.FindServiceByID(r.ID)
			if !ok {
				feedback.Warnf("brick %s requires service %s, but it was not found in the services index", brick.ID, r.ID)
				continue
			}
			if serviceComposeFile, ok := service.GetComposeFile(); ok {
				images, err := extractImagesFromCompose(serviceComposeFile)
				if err != nil {
					return nil, fmt.Errorf("failed to extract images from compose file of service %s required by brick %s: %w", r.ID, brick.ID, err)
				}
				result = append(result, images...)
			}
		}
	}

	return f.Uniq(result), nil
}

func extractImagesFromCompose(composeFile *paths.Path) ([]string, error) {
	var result []string

	content, err := composeFile.ReadFile()
	if err != nil {
		return nil, err
	}
	prj, err := loader.LoadWithContext(
		context.Background(),
		types.ConfigDetails{
			WorkingDir:  composeFile.Parent().String(),
			ConfigFiles: []types.ConfigFile{{Content: content}},
			Environment: types.NewMapping(os.Environ()),
		},
		func(o *loader.Options) { o.SetProjectName("default", false) },
		loader.WithSkipValidation, // avoid os.Getwd() in schema validation, which fails when CWD is missing (e.g. during .deb postinst)
	)
	if err != nil {
		return nil, err
	}
	for _, v := range prj.Services {
		if slices.ContainsFunc(imagePrefixes, func(p string) bool {
			return strings.HasPrefix(v.Image, p)
		}) {
			result = append(result, v.Image)
		} else {
			slog.Warn("skipping image that does not match known prefixes", "image", v.Image, "prefixes", imagePrefixes)
		}
	}
	return result, nil
}

type SystemCleanupResult struct {
	ContainersRemoved int
	NetworksRemoved   int
	ImagesRemoved     int
	RunningAppRemoved bool
	SpaceFreed        int64 // in bytes
}

func (s SystemCleanupResult) IsEmpty() bool {
	return s == SystemCleanupResult{}
}

// SystemCleanup removes dangling containers and unused images.
// Also running apps are stopped and removed.
func SystemCleanup(ctx context.Context, cfg config.Configuration, bricksindex *bricksindex.BricksIndex, servicesindex *servicesindex.ServicesIndex, modelsIndex *modelsindex.ModelsIndex, docker command.Cli, platform platform.Platform) (SystemCleanupResult, error) {
	var result SystemCleanupResult

	// Remove running app
	runningApp, err := getRunningApp(ctx, docker.Client())
	if err != nil {
		feedback.Warnf("failed to get running app - %v", err)
	}
	if runningApp != nil {
		err := StopAndDestroyApp(ctx, docker, platform, *runningApp, cfg, func(item StreamMessage) {})
		if err != nil {
			feedback.Warnf("failed to stop and destroy running app - %v", err)
		} else {
			result.RunningAppRemoved = true
		}
	}

	// Remove dangling stuff
	if count, err := removeDanglingContainers(ctx, docker.Client()); err != nil {
		feedback.Warnf("failed to remove dangling containers - %v", err)
	} else {
		result.ContainersRemoved = count
	}
	if count, err := removeDanglingNetworks(ctx, docker.Client()); err != nil {
		feedback.Warnf("failed to remove dangling networks - %v", err)
	} else {
		result.NetworksRemoved = count
	}

	// Remove unused images
	imagesMustStay, err := getRequiredImages(cfg, bricksindex, servicesindex, modelsIndex)
	if err != nil {
		return result, err
	}
	slog.Debug("images that must stay", "imagesMustStay", imagesMustStay)

	allImages, err := listImagesAlreadyPulled(ctx, docker.Client())
	if err != nil {
		return result, err
	}
	slog.Debug("all images already pulled", "allImages", allImages)

	imagesToRemove := slices.DeleteFunc(allImages, func(v string) bool {
		return slices.Contains(imagesMustStay, v)
	})
	slog.Info("images to remove", "imagesToRemove", imagesToRemove)

	for _, image := range imagesToRemove {
		imageSize, err := removeImage(ctx, docker.Client(), image)
		if err != nil {
			feedback.Warnf("failed to remove image %s - %v", image, err)
			continue
		}
		result.SpaceFreed += imageSize
		result.ImagesRemoved++
	}

	return result, nil
}

func removeImage(ctx context.Context, docker dockerClient.APIClient, imageName string) (int64, error) {
	var size int64
	if info, err := docker.ImageInspect(ctx, imageName); err != nil {
		feedback.Warnf("failed to inspect image %s - %v", imageName, err)
	} else {
		size = info.Size
	}

	if _, err := docker.ImageRemove(ctx, imageName, image.RemoveOptions{
		Force:         true,
		PruneChildren: true,
	}); err != nil {
		return 0, fmt.Errorf("failed to remove image %s: %w", imageName, err)
	}

	return size, nil
}

// images required by the system
func getRequiredImages(cfg config.Configuration, bricksindex *bricksindex.BricksIndex, servicesindex *servicesindex.ServicesIndex, modelsIndex *modelsindex.ModelsIndex) ([]string, error) {
	bricksContainers, err := getAllSupportedBrickImages(bricksindex, servicesindex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse bricks runner images: %w", err)
	}

	handlerImages := modelsIndex.Handlers.GetDockerImages()

	requiredImages := make([]string, 0, 1+len(bricksContainers)+len(handlerImages))
	requiredImages = append(requiredImages, cfg.PythonImage)
	requiredImages = append(requiredImages, bricksContainers...)
	requiredImages = append(requiredImages, handlerImages...)

	return requiredImages, nil
}

func removeDanglingContainers(ctx context.Context, docker dockerClient.APIClient) (int, error) {
	containers, err := docker.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", DockerAppLabel+"=true")),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list containers: %w", err)
	}

	var counter int
	for _, info := range containers {
		if err := docker.ContainerRemove(ctx, info.ID, container.RemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		}); err != nil {
			return 0, fmt.Errorf("failed to remove container %s: %w", info.ID, err)
		}
		counter++
	}

	return counter, nil
}

func removeDanglingNetworks(ctx context.Context, docker dockerClient.APIClient) (int, error) {
	const dockerComposeProjectLabel = "com.docker.compose.project"

	networks, err := docker.NetworkList(ctx, network.ListOptions{
		Filters: filters.NewArgs(filters.Arg("label", dockerComposeProjectLabel)),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list networks: %w", err)
	}

	var counter int
	for _, info := range networks {
		if !strings.Contains(info.Labels[dockerComposeProjectLabel], "arduino-app-cli") {
			continue
		}
		if err := docker.NetworkRemove(ctx, info.ID); err != nil {
			return 0, fmt.Errorf("failed to remove network %s: %w", info.ID, err)
		}
		counter++
	}

	return counter, nil
}

func installPlatformPackage(ctx context.Context, plat platform.Platform, eventCB InitEventCallback) error {
	var packageName string

	switch plat.BoardName {
	case "unoq":
		packageName = "arduino-unoq"
	case "ventunoq":
		packageName = "arduino-ventunoq"
	default:
		eventCB(InitEvent{Type: InitLogEvent, Source: InitSourceDeb, Message: fmt.Sprintf("no platform-specific debian package to install for board '%s'", plat.BoardName)})
		return nil
	}

	eventCB(InitEvent{Type: InitLogEvent, Source: InitSourceDeb, Message: fmt.Sprintf("Installing package '%s'", packageName)})

	cmd, err := paths.NewProcess(nil, "sudo", "apt-get", "install", "-y", packageName)
	if err != nil {
		return err
	}
	// Route the subprocess output through the event callback, one log event per line.
	subprocessOut := NewCallbackWriter(func(line string) {
		eventCB(InitEvent{Type: InitLogEvent, Source: InitSourceDeb, Message: line})
	})
	cmd.RedirectStderrTo(subprocessOut)
	cmd.RedirectStdoutTo(subprocessOut)

	if err := cmd.RunWithinContext(ctx); err != nil {
		return err
	}
	return nil
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
	totalSize := int64(0)
	downloadProgressCB := func(curr *rpc.DownloadProgress) {
		if start := curr.GetStart(); start != nil {
			currLabel = start.GetLabel()
		}
		if update := curr.GetUpdate(); update != nil {
			totalSize = update.GetTotalSize()
			eventCB(InitEvent{Type: InitProgressEvent, Source: InitSourceArduino, Progress: InitProgress{
				Label: currLabel,
				Curr:  update.GetDownloaded(),
				Total: totalSize,
			}})
		}
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

	// Install zephyr platform
	{
		if err := cli.Init(&rpc.InitRequest{Instance: cliInstance}, commands.InitStreamResponseToCallbackFunction(ctx, func(r *rpc.InitResponse) error {
			if p := r.GetInitProgress().GetDownloadProgress(); p != nil {
				downloadProgressCB(p)
			}
			return nil
		})); err != nil {
			return fmt.Errorf("could not initialize Arduino Core Server: %w", err)
		}

		str := commands.PlatformInstallStreamResponseToCallbackFunction(ctx, downloadProgressCB, func(msg *rpc.TaskProgress) {})
		if err := cli.PlatformInstall(&rpc.PlatformInstallRequest{
			Instance:        cliInstance,
			PlatformPackage: "arduino",
			Architecture:    "zephyr",
		}, str); err != nil {
			return fmt.Errorf("could not install zephyr platform: %w", err)
		}
	}

	// Get a list of example apps
	pathsToExplore := cfg.ExamplesDirs(platform)
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

	return nil
}

func downloadSketchLibsUsedInApp(ctx context.Context, appPath *paths.Path, platform platform.Platform, cli rpc.ArduinoCoreServiceServer, cliInstance *rpc.Instance, downloadProgressCB func(*rpc.DownloadProgress)) error {
	// Open the app to get the sketch path
	app, err := app.Load(appPath)
	if err != nil {
		return err
	}

	if ok, err := migrateRemoveRouterBridgeIfNeeded(ctx, platform, app); err != nil {
		slog.Warn("Failed to migrate app to remove router bridge", "app", appPath, "error", err)
	} else if ok {
		slog.Info("App migrated, RouterBridge has been removed successfully", "app", appPath)
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
