// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/moby/moby/api/types/container"
	"go.bug.st/f"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/dockerhelper"
	"github.com/arduino/arduino-app-cli/internal/helpers"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-app-cli/internal/sudo"
)

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
func SystemInit(ctx context.Context, cfg config.Configuration, platform platform.Platform, bricksindex *bricksindex.BricksIndex, servicesindex *servicesindex.ServicesIndex, docker dockerhelper.Cli, modelsIndex *modelsindex.ModelsIndex, options SystemInitOptions, eventCB InitEventCallback) error {
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

func downloadSupportedImages(ctx context.Context, cfg config.Configuration, brickindex *bricksindex.BricksIndex, servicesindex *servicesindex.ServicesIndex, modelsIndex *modelsindex.ModelsIndex, docker dockerhelper.Cli, eventCB InitEventCallback) error {
	eventCB(InitEvent{Type: InitLogEvent, Source: InitSourceDocker, Message: "Pulling the latest docker images ..."})
	brickImages, err := getAllSupportedBrickImages(brickindex, servicesindex)
	if err != nil {
		return err
	}
	handlerImages := modelsIndex.Handlers.GetDockerImages()

	imagesToPreinstall := make([]string, 0, 1+len(brickImages)+len(handlerImages))
	imagesToPreinstall = append(imagesToPreinstall, cfg.PythonImage)
	imagesToPreinstall = append(imagesToPreinstall, brickImages...)
	imagesToPreinstall = append(imagesToPreinstall, handlerImages...)

	return dockerhelper.PullImages(ctx, docker.Client(), imagesToPreinstall,
		func(line string) {
			eventCB(InitEvent{Type: InitLogEvent, Source: InitSourceDocker, Message: line})
		},
		func(label string, curr, total int64) {
			eventCB(InitEvent{Type: InitProgressEvent, Source: InitSourceDocker, Progress: InitProgress{Label: label, Curr: curr, Total: total}})
		})
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
		if isOurImage(v.Image) {
			result = append(result, v.Image)
		} else {
			slog.Warn("skipping image that is not one of ours", "image", v.Image)
		}
	}
	return result, nil
}

// isOurImage states if an image is one of ours, past ones included: what to pull, and
// what a cleanup may remove.
func isOurImage(image string) bool {
	prefixes := []string{
		"ghcr.io/bcmi-labs/",
		"public.ecr.aws/arduino/",
		"ghcr.io/arduino/",
		"influxdb",
		"artifacts.codelinaro.org/iot-solutions-microservices/",
	}
	return slices.ContainsFunc(prefixes, func(p string) bool { return strings.HasPrefix(image, p) })
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

type SystemCleanupOptions struct {
	// Releases removes the images of the installed releases too.
	Releases bool
}

// SystemCleanup removes dangling containers and unused images.
// Also running apps are stopped and removed.
func SystemCleanup(ctx context.Context, cfg config.Configuration, bricksindex *bricksindex.BricksIndex, servicesindex *servicesindex.ServicesIndex, modelsIndex *modelsindex.ModelsIndex, docker dockerhelper.Cli, platform platform.Platform, opts SystemCleanupOptions) (SystemCleanupResult, error) {
	var result SystemCleanupResult

	// Read before anything removes the containers: they tell which network is ours.
	appContainers, err := dockerhelper.Containers(ctx, docker.Client(), DockerAppLabel+"=true")
	if err != nil {
		feedback.Warnf("failed to list the app containers - %v", err)
	}
	removeNetwork := ourNetworks(appContainers)

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
	if count, err := dockerhelper.PruneContainers(ctx, docker.Client(), DockerAppLabel+"=true", nil); err != nil {
		feedback.Warnf("failed to remove dangling containers - %v", err)
	} else {
		result.ContainersRemoved = count
	}
	// A network that resists does not cancel the others: the count is read anyway.
	count, err := dockerhelper.PruneNetworks(ctx, docker.Client(), removeNetwork)
	if err != nil {
		feedback.Warnf("failed to remove dangling networks - %v", err)
	}
	result.NetworksRemoved = count

	// Remove unused images
	imagesMustStay, err := getRequiredImages(cfg, bricksindex, servicesindex, modelsIndex)
	if err != nil {
		return result, err
	}
	if !opts.Releases {
		// A release that cannot be read can lose its images: its start fails until a prepare.
		releaseImages, err := getReleaseImages(cfg)
		if err != nil {
			feedback.Warnf("the images of some releases may be removed - %v", err)
		}
		imagesMustStay = append(imagesMustStay, releaseImages...)
	}
	slog.Debug("images that must stay", "imagesMustStay", imagesMustStay)

	allImages, err := dockerhelper.ListImages(ctx, docker.Client())
	if err != nil {
		return result, err
	}
	slog.Debug("all images already pulled", "allImages", allImages)

	imagesToRemove := removableImages(allImages, imagesMustStay)
	slog.Info("images to remove", "imagesToRemove", imagesToRemove)

	for _, image := range imagesToRemove {
		imageSize, err := dockerhelper.RemoveImage(ctx, docker.Client(), image)
		if err != nil {
			feedback.Warnf("failed to remove image %s - %v", image, err)
			continue
		}
		result.SpaceFreed += imageSize
		result.ImagesRemoved++
	}

	return result, nil
}

// removableImages is what a cleanup may delete: our images, minus the ones in use.
func removableImages(allImages, imagesMustStay []string) []string {
	return slices.DeleteFunc(slices.Clone(allImages), func(image string) bool {
		return !isOurImage(image) || slices.Contains(imagesMustStay, image)
	})
}

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

// getReleaseImages lists the images of the installed releases, from the compose file
// their install rendered. The error joins the releases it cannot read; the images of
// the others are still returned.
func getReleaseImages(cfg config.Configuration) ([]string, error) {
	releasePaths, errs := app.FindAppsInFolders(paths.NewPathList(cfg.ReleasesDir().String()))
	var images []string
	for _, releasePath := range releasePaths {
		release, err := app.Load(releasePath)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("cannot load the release %s: %w", releasePath.Base(), err))
			continue
		}
		releaseImages, err := extractImagesFromCompose(release.AppComposeFilePath())
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("cannot read the images of the release %s: %w", releasePath.Base(), err))
			continue
		}
		images = append(images, releaseImages...)
	}
	return images, errs
}

func installPlatformPackage(ctx context.Context, plat platform.Platform, eventCB InitEventCallback) error {
	if !slices.Contains(platform.SupportedBoards(), plat.BoardName) {
		eventCB(InitEvent{Type: InitLogEvent, Source: InitSourceDeb, Message: fmt.Sprintf("no platform-specific debian package to install for board '%s'", plat.BoardName)})
		return nil
	}
	packageName := platform.DebianPackage(plat.BoardName)

	eventCB(InitEvent{Type: InitLogEvent, Source: InitSourceDeb, Message: fmt.Sprintf("Installing package '%s'", packageName)})

	cmd, err := sudo.AptInstall.Process(packageName)
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

// platformVersionToInstall returns a platform version to install if it isn't installed, "" otherwise.
func platformVersionToInstall(
	ctx context.Context,
	cli rpc.ArduinoCoreServiceServer,
	inst *rpc.Instance,
	platformID string,
	constraint semver.Constraint,
) (string, error) {
	platforms, err := cli.PlatformSearch(ctx, &rpc.PlatformSearchRequest{
		Instance:          inst,
		ManuallyInstalled: true,
	})
	if err != nil {
		return "", fmt.Errorf("could not search the %s platform: %w", platformID, err)
	}

	var summary *rpc.PlatformSummary
	for _, p := range platforms.GetSearchOutput() {
		if p.GetMetadata().GetId() == platformID {
			summary = p
			break
		}
	}
	if summary == nil {
		return "", fmt.Errorf("platform %s not found in the platforms index", platformID)
	}

	if summary.GetInstalledVersion() != "" {
		return "", nil // A platform is already installed, return an empty version string.
	}

	available := make([]string, 0, len(summary.GetReleases()))
	for version := range summary.GetReleases() {
		available = append(available, version)
	}

	best := helpers.SelectBestVersion(available, nil, constraint)
	if best == nil {
		return "", fmt.Errorf("no version of platform %s satisfies the constraint '%s'", platformID, constraint)
	}
	return best.String(), nil
}

// composeProjectLabel names the project a container or a network belongs to.
const composeProjectLabel = "com.docker.compose.project"

// ourNetworks tells the networks of our apps, which cost a subnet each. The label
// reaches a network only from this version on: an older one is ours by the compose
// project of the containers, which the caller reads before they go.
func ourNetworks(containers []container.Summary) func(labels map[string]string) bool {
	projects := map[string]bool{}
	for _, info := range containers {
		// An empty key would take every network that carries no project either.
		if project := info.Labels[composeProjectLabel]; project != "" {
			projects[project] = true
		}
	}

	return func(labels map[string]string) bool {
		return labels[DockerAppLabel] == "true" || projects[labels[composeProjectLabel]]
	}
}
