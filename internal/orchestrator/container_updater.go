package orchestrator

import (
	"context"
	"slices"

	"github.com/docker/cli/cli/command"
	semver "go.bug.st/relaxed-semver"

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

// isNewerVersion returns true if newImage has a strictly higher semver tag than currentImage.
// Both arguments are expected to share the same image base name.
// If either tag cannot be parsed as semver, it returns true (treat as upgrade to be safe).
func isNewerVersion(newImage, currentImage string) bool {
	_, newTag := parseDockerImage(newImage)
	_, currentTag := parseDockerImage(currentImage)

	newVer, err := semver.Parse(newTag)
	if err != nil {
		return true
	}
	currentVer, err := semver.Parse(currentTag)
	if err != nil {
		return true
	}
	return currentVer.LessThan(newVer)
}
