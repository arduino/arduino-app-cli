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
	requiredImages := []string{c.cfg.PythonImage}
	brickImages, err := getAllSupportedBrickImages(c.bricksIndex, c.servicesIndex)
	if err != nil {
		return nil, err
	}
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
