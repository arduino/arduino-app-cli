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
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

var ErrDockerOutOfSpace = errors.New("not enough disk space to pull the docker image")

const ExitCodeDockerOutOfSpace = 80

type initProgress struct {
	label string
	curr  int64
	total int64
}

type initProgressCallback func(progress initProgress)

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
func SystemInit(ctx context.Context, cfg config.Configuration, bricksindex *bricksindex.BricksIndex, servicesindex *servicesindex.ServicesIndex, docker *command.DockerCli, options SystemInitOptions) error {
	if err := options.Validate(); err != nil {
		return err
	}

	stdout, _, err := feedback.DirectStreams()
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrBadArgument)
		return nil
	}

	// TODO: Move this callback up in the call chain, closer to Cobra command definition
	progressCB := func(progress initProgress) {
		percentage := float64(progress.curr) / float64(progress.total) * 100
		fmt.Fprintf(stdout, "%s: %.2f%% (%d/%d)\r", progress.label, percentage, progress.curr, progress.total)
		if progress.curr == progress.total {
			fmt.Fprintln(stdout)
		}
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

	if downloadPlatformAndLibs {
		if err := downloadLibsAndPlatformsUsedInExamples(ctx, cfg, progressCB); err != nil {
			return fmt.Errorf("failed to download libs and platforms used in examples: %w", err)
		}
	}

	if downloadDockerImages {
		// TODO: use progressCB instead of stdout
		if err := downloadSupportedImages(ctx, cfg, bricksindex, servicesindex, docker, stdout); err != nil {
			return fmt.Errorf("failed to download container images used in examples: %w", err)
		}
	}

	return nil
}

func downloadSupportedImages(ctx context.Context, cfg config.Configuration, brickindex *bricksindex.BricksIndex, servicesindex *servicesindex.ServicesIndex, docker *command.DockerCli, stdout io.Writer) error {
	imagesToPreinstall := []string{cfg.PythonImage}
	brickImages, err := getAllSupportedBrickImages(brickindex)
	if err != nil {
		return err
	}
	imagesToPreinstall = append(imagesToPreinstall, brickImages...)

	brickServiceImages, err := getAllSupportedBrickServiceImages(servicesindex)
	if err != nil {
		return err
	}
	imagesToPreinstall = append(imagesToPreinstall, brickServiceImages...)

	pulledImages, err := listImagesAlreadyPulled(ctx, docker.Client())
	if err != nil {
		return err
	}

	// Filter out container images that are alredy pulled
	imagesToPreinstall = slices.DeleteFunc(imagesToPreinstall, func(v string) bool {
		return slices.Contains(pulledImages, v)
	})

	for _, image := range imagesToPreinstall {
		freeSpace, err := GetDockerFreeSpace()
		if err != nil {
			return err
		}

		// Check that there is enough disk space for the additional layers needed by the image.
		previousExistingImage := GetHighestVersion(image, pulledImages)
		if toDownload, err := GetBytesToDownload(previousExistingImage, image, stdout); err != nil {
			// In case of errors getting the size to download, proceed anyway.
			slog.Warn("Unable to get the new image layers size", "image", image, "error", err)
		} else if uint64(float64(toDownload)*2.5) > freeSpace {
			return ErrDockerOutOfSpace
		}

		feedback.Printf("Pulling container image %s ...", image)
		if err := pullImage(ctx, stdout, docker.Client(), image); err != nil {
			return fmt.Errorf("failed to pull image %s: %w", image, err)
		}
	}

	return nil
}

const minDelay = 1 * time.Second
const maxDelay = 10 * time.Second

func pullImage(ctx context.Context, stdout io.Writer, docker dockerClient.APIClient, imageName string) error {
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

		feedback.Warnf("received 'toomanyrequests' error from Docker registry, retrying in %s ...", delay)

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
			Status   string `json:"status"`
			Progress string `json:"progress"`
			ID       string `json:"id"`
		}

		var payload Payload
		if err := json.Unmarshal(scanner.Bytes(), &payload); err == nil {
			if payload.Status != "" {
				fmt.Fprintf(stdout, "%s", payload.Status)
			}
			if payload.Progress != "" {
				fmt.Fprintf(stdout, "[%s] %s\r", payload.ID, payload.Progress)
			} else {
				fmt.Fprintln(stdout)
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
var imagePrefixes = []string{"ghcr.io/bcmi-labs/", "public.ecr.aws/arduino/", "ghcr.io/arduino/", "influxdb"}

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
			for _, prefix := range imagePrefixes {
				if strings.HasPrefix(tag, prefix) {
					result = append(result, tag)
				}
			}
		}
	}

	return result, nil
}

func getAllSupportedBrickImages(bricksIndex *bricksindex.BricksIndex) ([]string, error) {
	var result []string
	for _, brick := range bricksIndex.ListBricks() {
		composeFile, ok := brick.GetComposeFile()
		if !ok {
			continue
		}
		images, err := extractImagesFromCompose(composeFile)
		if err != nil {
			return nil, err
		}
		result = append(result, images...)
	}

	return f.Uniq(result), nil
}

func getAllSupportedBrickServiceImages(serviceIndex *servicesindex.ServicesIndex) ([]string, error) {
	var result []string
	for _, service := range serviceIndex.Services {
		composeFile, ok := service.GetComposeFile()
		if !ok {
			continue
		}
		images, err := extractImagesFromCompose(composeFile)
		if err != nil {
			return nil, err
		}
		result = append(result, images...)
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
			ConfigFiles: []types.ConfigFile{{Content: content}},
			Environment: types.NewMapping(os.Environ()),
		},
		func(o *loader.Options) { o.SetProjectName("default", false) },
	)
	if err != nil {
		return nil, err
	}
	for _, v := range prj.Services {
		for _, prefix := range imagePrefixes {
			if strings.HasPrefix(v.Image, prefix) {
				result = append(result, v.Image)
			}
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
func SystemCleanup(ctx context.Context, cfg config.Configuration, bricksindex *bricksindex.BricksIndex, servicesindex *servicesindex.ServicesIndex, docker command.Cli, platform platform.Platform) (SystemCleanupResult, error) {
	var result SystemCleanupResult

	// Remove running app
	runningApp, err := getRunningApp(ctx, docker.Client())
	if err != nil {
		feedback.Warnf("failed to get running app - %v", err)
	}
	if runningApp != nil {
		for item := range StopAndDestroyApp(ctx, docker, platform, *runningApp) {
			if item.GetType() == ErrorType {
				feedback.Warnf("failed to stop and destroy running app - %v", item.GetError())
				break
			}
		}
		result.RunningAppRemoved = true
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
	containersMustStay, err := getRequiredImages(cfg, bricksindex, servicesindex)
	if err != nil {
		return result, err
	}
	allImages, err := listImagesAlreadyPulled(ctx, docker.Client())
	if err != nil {
		return result, err
	}
	imagesToRemove := slices.DeleteFunc(allImages, func(v string) bool {
		return slices.Contains(containersMustStay, v)
	})

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

// imgages required by the system
func getRequiredImages(cfg config.Configuration, bricksindex *bricksindex.BricksIndex, servicesindex *servicesindex.ServicesIndex) ([]string, error) {
	bricksContainers, err := getAllSupportedBrickImages(bricksindex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse bricks runner images: %w", err)
	}

	bricksServiceContainers, err := getAllSupportedBrickServiceImages(servicesindex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse brick services images: %w", err)
	}

	requiredImages := make([]string, 0, 1+len(bricksContainers)+len(bricksServiceContainers))
	requiredImages = append(requiredImages, cfg.PythonImage)
	requiredImages = append(requiredImages, bricksContainers...)
	requiredImages = append(requiredImages, bricksServiceContainers...)

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

func downloadLibsAndPlatformsUsedInExamples(ctx context.Context, cfg config.Configuration, progressCB initProgressCallback) error {
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
			progressCB(initProgress{
				label: currLabel,
				curr:  update.GetDownloaded(),
				total: totalSize,
			})
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
	exampleAppsPath, err := app.FindAppsInFolder(cfg.ExamplesDir())
	if err != nil {
		return err
	}

	// After downloading the libs, clean up the download cache
	defer func() {
		_, _ = cli.CleanDownloadCacheDirectory(ctx, &rpc.CleanDownloadCacheDirectoryRequest{Instance: cliInstance})
	}()

	// Download libraries used in each example app
	for _, appPath := range exampleAppsPath {
		if err := downloadSketchLibsUsedInApp(ctx, appPath, cli, cliInstance, downloadProgressCB); err != nil {
			return fmt.Errorf("could not download libs in app %s: %w", appPath, err)
		}
	}

	return nil
}

func downloadSketchLibsUsedInApp(ctx context.Context, appPath *paths.Path, cli rpc.ArduinoCoreServiceServer, cliInstance *rpc.Instance, downloadProgressCB func(*rpc.DownloadProgress)) error {
	// Open the app to get the sketch path
	app, err := app.Load(appPath)
	if err != nil {
		return err
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
