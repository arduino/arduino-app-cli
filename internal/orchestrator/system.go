package orchestrator

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	dockerClient "github.com/docker/docker/client"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/store"
)

// SystemInit pulls necessary Docker images.
func SystemInit(ctx context.Context, cfg config.Configuration, staticStore *store.StaticStore, docker *command.DockerCli) error {
	containersToPreinstall := []string{cfg.PythonImage}
	additionalContainers, err := parseAllModelsRunnerImageTag(staticStore)
	if err != nil {
		return err
	}
	containersToPreinstall = append(containersToPreinstall, additionalContainers...)

	pulledImages, err := listImagesAlreadyPulled(ctx, docker.Client())
	if err != nil {
		return err
	}

	// Filter out containers alredy pulled
	containersToPreinstall = slices.DeleteFunc(containersToPreinstall, func(v string) bool {
		return slices.Contains(pulledImages, v)
	})

	stdout, _, err := feedback.DirectStreams()
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrBadArgument)
		return nil
	}

	for _, container := range containersToPreinstall {
		feedback.Printf("Pulling container image %s ...", container)
		if err := pullImage(ctx, stdout, docker.Client(), container); err != nil {
			feedback.Printf("Warning: failed to read image pull response - %v", err)
		}
	}

	return nil
}

func pullImage(ctx context.Context, stdout io.Writer, docker dockerClient.APIClient, imageName string) error {
	delay := 1 * time.Second

	var out io.ReadCloser
	var allErr error
	var lastErr error
	for range 4 { // 1s, 2s, 4s, 8s
		out, lastErr = docker.ImagePull(ctx, imageName, image.PullOptions{})
		if lastErr == nil {
			break // Success
		}
		if !strings.Contains(lastErr.Error(), "toomanyrequests") {
			return lastErr // Fail fast on non-rate-limit errors
		}
		allErr = errors.Join(allErr, lastErr)

		feedback.Printf("Warning: received 'toomanyrequests' error from Docker registry, retrying in %s ...", delay)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		delay *= 2
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

// Container images matching this list will be pulled by 'system init' and included in the Linux images.
var imagePrefixes = []string{"ghcr.io/bcmi-labs/", "public.ecr.aws/arduino/", "influxdb"}

// listImagesAlreadyPulled
// TODO make reference constant in a dedicated file as single source of truth
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

func parseAllModelsRunnerImageTag(staticStore *store.StaticStore) ([]string, error) {
	composePath := staticStore.GetComposeFolder()
	brickNamespace := "arduino"
	bricks, err := composePath.Join(brickNamespace).ReadDir()
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(bricks))
	for _, brick := range bricks {
		composeFile := composePath.Join(brickNamespace, brick.Base(), "brick_compose.yaml")
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
			func(o *loader.Options) { o.SetProjectName("test", false) },
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
	}

	return f.Uniq(result), nil
}

type SystemCleanupResult struct {
	ContainersRemoved int
	ImagesRemoved     int
	RunningAppRemoved bool
	SpaceFreed        int64 // in bytes
}

func (s SystemCleanupResult) IsEmpty() bool {
	return s == SystemCleanupResult{}
}

// SystemCleanup removes dangling containers and unused images.
// Also running apps are stopped and removed.
func SystemCleanup(ctx context.Context, cfg config.Configuration, staticStore *store.StaticStore, docker command.Cli) (SystemCleanupResult, error) {
	var result SystemCleanupResult

	// Remove running app and dangling containers
	runningApp, err := getRunningApp(ctx, docker.Client())
	if err != nil {
		feedback.Printf("Warning: failed to get running app - %v", err)
	}
	if runningApp != nil {
		for item := range StopAndDestroyApp(ctx, *runningApp) {
			if item.GetType() == ErrorType {
				feedback.Printf("Warning: failed to stop and destroy running app - %v", item.GetError())
				break
			}
		}
		result.RunningAppRemoved = true
	}
	if count, err := removeDanglingContainers(ctx, docker.Client()); err != nil {
		feedback.Printf("Warning: failed to remove dangling containers - %v", err)
	} else {
		result.ContainersRemoved = count
	}

	// Remove unused images
	containersMustStay, err := getRequiredImages(cfg, staticStore)
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
			feedback.Printf("Warning: failed to remove image %s - %v", image, err)
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
		feedback.Printf("Warning: failed to inspect image %s - %v", imageName, err)
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
func getRequiredImages(cfg config.Configuration, staticStore *store.StaticStore) ([]string, error) {
	requiredImages := []string{cfg.PythonImage}

	modelsRunnersContainers, err := parseAllModelsRunnerImageTag(staticStore)
	if err != nil {
		return nil, fmt.Errorf("failed to parse models runner images: %w", err)
	}

	requiredImages = append(requiredImages, modelsRunnersContainers...)
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
