// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"slices"

	"github.com/docker/cli/cli/command"

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

func (c *ContainerUpdater) ListUpgradableImages(ctx context.Context) ([]update.UpgradableImage, error) {
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

	var result []update.UpgradableImage
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
		result = append(result, update.UpgradableImage{
			ToVersion:   img,
			FromVersion: current,
		})
	}

	return result, nil
}
