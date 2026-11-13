// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/arduino/go-paths-helper"
	"github.com/docker/cli/cli/command"
	dockerClient "github.com/moby/moby/client"
	"go.bug.st/downloader/v3"
	"go.bug.st/f"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/dockerhelper"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

// BrickImages returns the images the brick runs on the board: the ones of its compose file and
// of the services it requires.
func BrickImages(brick bricksindex.Brick, servicesIndex *servicesindex.ServicesIndex) ([]string, error) {
	images, err := brickComposeImages(brick)
	if err != nil {
		return nil, err
	}
	for _, r := range brick.RequiresServices {
		service, ok := servicesIndex.FindServiceByID(r.ID)
		if !ok {
			slog.Warn("service required by brick not found", "service_id", r.ID, "brick_id", brick.ID)
			continue
		}
		if composeFile, ok := service.GetComposeFile(); ok {
			serviceImages, err := composeImages(composeFile)
			if err != nil {
				return nil, fmt.Errorf("invalid compose file of the service %s: %w", r.ID, err)
			}
			images = append(images, serviceImages...)
		}
	}
	return f.Uniq(images), nil
}

func brickComposeImages(brick bricksindex.Brick) ([]string, error) {
	composeFile, ok := brick.GetComposeFile()
	if !ok {
		return nil, nil
	}
	images, err := composeImages(composeFile)
	if err != nil {
		return nil, fmt.Errorf("invalid compose file of the brick %s: %w", brick.ID, err)
	}
	return images, nil
}

// installedBrickImages returns the images of every installed version of the bricks installed
// from the bricks index. A version whose images cannot be read is skipped: it cannot run.
func installedBrickImages(bricksIndex *bricksindex.BricksIndex, servicesIndex *servicesindex.ServicesIndex) []string {
	var res []string
	for _, brick := range bricksIndex.AllInstalledBricks() {
		images, err := BrickImages(brick, servicesIndex)
		if err != nil {
			slog.Warn("cannot read the images of an installed brick", "brick", brick.ID+"@"+brick.Version, "error", err)
			continue
		}
		res = append(res, images...)
	}
	return res
}

// InstallBrick installs the release as bricksindex.Install does, pulling the images of the brick
// before it is installed: a brick whose images cannot be pulled is not installed.
func InstallBrick(
	ctx context.Context,
	docker command.Cli,
	downloadConfig downloader.Config,
	bricksDir *paths.Path,
	servicesIndex *servicesindex.ServicesIndex,
	brick *bricksindex.RemoteBrick,
	release *bricksindex.RemoteRelease,
	platform platform.Platform,
	onDownload func(downloaded, total int64),
	onWait func(),
	onPullLine func(string),
	onPullProgress func(label string, curr, total int64),
) (bricksindex.InstallResult, error) {
	pull := func(b bricksindex.Brick) error {
		images, err := BrickImages(b, servicesIndex)
		if err != nil {
			return err
		}
		return dockerhelper.PullImages(ctx, docker.Client(), images, onPullLine, onPullProgress)
	}
	return bricksindex.Install(ctx, downloadConfig, bricksDir, brick, release, platform, onDownload, onWait, pull)
}

// UninstallBrick uninstalls the brick as bricksindex.Uninstall does, then removes the images of the
// uninstalled versions that nothing else on the board uses. An image that cannot be removed is
// reported to warn and left on the board.
func UninstallBrick(
	ctx context.Context,
	docker command.Cli,
	cfg config.Configuration,
	bricksIndex *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
	modelsIndex *modelsindex.ModelsIndex,
	id string,
	version *semver.Version,
	onWait func(),
	warn func(string),
) (bricksindex.UninstallResult, error) {
	// Read before the files are removed. The images of the required services are ours, and
	// the system cleanup removes them once nothing needs them.
	imagesByVersion := map[string][]string{}
	for _, brick := range bricksIndex.AllInstalledBricks() {
		if brick.ID != id || (version != nil && brick.Version != version.String()) {
			continue
		}
		images, err := brickComposeImages(brick)
		if err != nil {
			warn(fmt.Sprintf("the images of %s@%s are not removed: %v", id, brick.Version, err))
			continue
		}
		imagesByVersion[brick.Version] = images
	}

	res, err := bricksindex.Uninstall(ctx, cfg.BricksDir(), id, version, onWait)
	var images []string
	for _, v := range res.Versions {
		images = append(images, imagesByVersion[v]...)
	}
	if len(images) > 0 {
		removeUnusedImages(ctx, docker.Client(), cfg, bricksIndex, servicesIndex, modelsIndex, f.Uniq(images), warn)
	}
	return res, err
}

// removeUnusedImages removes the images that nothing on the board may run.
func removeUnusedImages(
	ctx context.Context,
	docker dockerClient.APIClient,
	cfg config.Configuration,
	bricksIndex *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
	modelsIndex *modelsindex.ModelsIndex,
	images []string,
	warn func(string),
) {
	inUse, err := imagesInUse(cfg, bricksIndex, servicesIndex, modelsIndex)
	if err != nil {
		warn(fmt.Sprintf("the images of the brick are not removed: %v", err))
		return
	}
	// The same image can be named by different references: a tag, a digest, or both.
	idsInUse, err := imageIDs(ctx, docker, inUse)
	if err != nil {
		warn(fmt.Sprintf("the images of the brick are not removed: %v", err))
		return
	}
	for _, image := range images {
		id, err := dockerhelper.ImageID(ctx, docker, image)
		if err != nil {
			warn(err.Error())
			continue
		}
		if id == "" || idsInUse[id] {
			continue
		}
		if err := dockerhelper.RemoveImageIfUnused(ctx, docker, image); err != nil {
			warn(err.Error())
		}
	}
}

// imagesInUse returns the images that something on the board may run: the python image, the
// images of the models, of the bricks, every installed version included, of their services, and
// of the installed releases.
func imagesInUse(cfg config.Configuration, bricksIndex *bricksindex.BricksIndex, servicesIndex *servicesindex.ServicesIndex, modelsIndex *modelsindex.ModelsIndex) ([]string, error) {
	images := []string{cfg.PythonImage}
	images = append(images, modelsIndex.Handlers.GetDockerImages()...)
	for _, brick := range bricksIndex.ListBricks() {
		brickImages, err := BrickImages(brick, servicesIndex)
		if err != nil {
			return nil, err
		}
		images = append(images, brickImages...)
	}
	images = append(images, installedBrickImages(bricksIndex, servicesIndex)...)
	// A release that cannot be read may run any image.
	releaseImages, err := getReleaseImages(cfg)
	if err != nil {
		return nil, err
	}
	return append(images, releaseImages...), nil
}

// imageIDs returns the ids of the images the engine has among the given ones.
func imageIDs(ctx context.Context, docker dockerClient.APIClient, images []string) (map[string]bool, error) {
	ids := map[string]bool{}
	for _, image := range f.Uniq(images) {
		id, err := dockerhelper.ImageID(ctx, docker, image)
		if err != nil {
			return nil, err
		}
		if id != "" {
			ids[id] = true
		}
	}
	return ids, nil
}
