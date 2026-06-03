// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"slices"
	"sync"

	"github.com/docker/cli/cli/command"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/update"
)

type ContainerUpdater struct {
	cfg           config.Configuration
	bricksIndex   *bricksindex.BricksIndex
	servicesIndex *servicesindex.ServicesIndex
	docker        *command.DockerCli
	lock          sync.Mutex
}

func NewContainerUpdater(
	cfg config.Configuration,
	bricksIndex *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
	docker *command.DockerCli,
) *ContainerUpdater {
	return &ContainerUpdater{
		cfg:           cfg,
		bricksIndex:   bricksIndex,
		servicesIndex: servicesIndex,
		docker:        docker,
	}
}

func (c *ContainerUpdater) ListUpgradableImages(ctx context.Context) ([]update.UpgradablePackage, error) {
	brickImages, err := getAllSupportedBrickImages(c.bricksIndex, c.servicesIndex)
	if err != nil {
		return nil, err
	}

	requiredImages := make([]string, 0, 1+len(brickImages))
	requiredImages = append(requiredImages, c.cfg.PythonImage)
	requiredImages = append(requiredImages, brickImages...)

	pulledImages, err := listImagesAlreadyPulled(ctx, c.docker.Client())
	if err != nil {
		return nil, err
	}

	var result []update.UpgradablePackage
	for _, img := range requiredImages {
		// If the exact image (same tag) is already present locally, nothing to do.
		if slices.Contains(pulledImages, img) {
			continue
		}

		// Otherwise, the image is either new or a newer version of an already installed one.
		current := GetHighestVersion(img, pulledImages)

		if current != "" && !isNewerVersion(img, current) {
			continue
		}
		baseName, _ := parseDockerImage(img)
		result = append(result, update.UpgradablePackage{
			Type:        update.Container,
			Name:        baseName,
			FromVersion: current,
			ToVersion:   img,
		})
	}

	return result, nil
}

func (s *ContainerUpdater) UpgradePackages(ctx context.Context, packages []update.PackageInfo, eventCB update.EventCallback) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	for line, err := range runSystemInit(ctx) {
		if err != nil {
			// In case of errors, including "out of disk space" erros, do a cleanup and then retry once.

			eventCB(update.NewDataEvent(update.UpgradeLineEvent, "Stop and destroy docker containers and images, to free up space ..."))
			streamCleanup := cleanupDockerContainers(ctx)
			for line, err := range streamCleanup {
				if err != nil {
					slog.Warn("Error during cleanup of container and images", "error", err)
				} else {
					eventCB(update.NewDataEvent(update.UpgradeLineEvent, line))
				}
			}

			// Try again to pull the docker containers.
			eventCB(update.NewDataEvent(update.UpgradeLineEvent, "Pulling the latest docker images (again) ..."))
			for line, err := range runSystemInit(ctx) {
				if err != nil {
					return fmt.Errorf("error pulling docker images: %w", err)
				}
				eventCB(update.NewDataEvent(update.UpgradeLineEvent, line))
			}
		} else {
			eventCB(update.NewDataEvent(update.UpgradeLineEvent, line))
		}
	}

	// After pulling new images is completed, remove old images to free up space.
	eventCB(update.NewDataEvent(update.UpgradeLineEvent, "Cleanup docker containers and images, to remove old unused images"))
	streamCleanup := cleanupDockerContainers(ctx)
	for line, err := range streamCleanup {
		if err != nil {
			slog.Warn("Error during cleanup of container and images", "error", err)
		} else {
			eventCB(update.NewDataEvent(update.UpgradeLineEvent, line))
		}
	}

	return nil
}
func runSystemInit(ctx context.Context) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		cmd, err := paths.NewProcess(nil, "arduino-app-cli", "system", "init")
		if err != nil {
			_ = yield("", err)
			return
		}

		stdout := NewCallbackWriter(func(line string) {
			if !yield(line, nil) {
				if err := cmd.Kill(); err != nil {
					slog.Error("Failed to kill 'arduino-app-cli system init' command", slog.String("error", err.Error()))
				}
			}
		})
		cmd.RedirectStderrTo(stdout)
		cmd.RedirectStdoutTo(stdout)

		if err = cmd.RunWithinContext(ctx); err != nil {
			_ = yield("", err)
			return
		}
	}
}

// Remove all stopped containers
func cleanupDockerContainers(ctx context.Context) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		cmd, err := paths.NewProcess(nil, "arduino-app-cli", "system", "cleanup")
		if err != nil {
			_ = yield("", err)
			return
		}

		stdout := NewCallbackWriter(func(line string) {
			if !yield(line, nil) {
				if err := cmd.Kill(); err != nil {
					slog.Error("Failed to kill 'arduino-app-cli system cleanup' command", slog.String("error", err.Error()))
				}
			}
		})
		cmd.RedirectStderrTo(stdout)
		cmd.RedirectStdoutTo(stdout)

		if err = cmd.RunWithinContext(ctx); err != nil {
			_ = yield("", err)
			return
		}
	}
}
