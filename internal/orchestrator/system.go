package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/store"
)

// SystemInit pulls necessary Docker images.
func SystemInit(ctx context.Context, cfg config.Configuration, staticStore *store.StaticStore) error {
	containersToPreinstall := []string{cfg.PythonImage}
	additionalContainers, err := parseAllModelsRunnerImageTag(staticStore)
	if err != nil {
		return err
	}
	containersToPreinstall = append(containersToPreinstall, additionalContainers...)

	pulledImages, err := listImagesAlreadyPulled(ctx)
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

		cmd, err := paths.NewProcess(nil, "docker", "pull", container)
		if err != nil {
			return err
		}
		cmd.RedirectStderrTo(stdout)
		cmd.RedirectStdoutTo(stdout)
		if err := cmd.RunWithinContext(ctx); err != nil {
			return err
		}
	}
	return nil
}

// listImagesAlreadyPulled
// TODO make reference constant in a dedicated file as single source of truth
func listImagesAlreadyPulled(ctx context.Context) ([]string, error) {
	cmd, err := paths.NewProcess(nil,
		"docker", "images", "--format", "json",
		"-f", "reference=ghcr.io/bcmi-labs/*",
		"-f", "reference=public.ecr.aws/arduino/app-bricks/*",
		"-f", "reference=influxdb",
	)
	if err != nil {
		return nil, err
	}

	// Capture the output to check if the image exists
	stdout, _, err := cmd.RunAndCaptureOutput(ctx)
	if err != nil {
		return nil, err
	}

	type dockerImage struct {
		Repository string `json:"Repository"`
		Tag        string `json:"Tag"`
	}
	var resp dockerImage
	result := []string{}
	for img := range bytes.Lines(stdout) {
		if len(img) == 0 {
			continue
		}
		if err := json.Unmarshal(img, &resp); err != nil {
			return nil, err
		}
		if resp.Tag == "<none>" {
			continue
		}
		result = append(result, resp.Repository+":"+resp.Tag)
	}

	return result, nil
}

// Container images matching this list will be pulled by 'system init' and included in the Linux images.
var imagePrefixes = []string{"ghcr.io/bcmi-labs/", "public.ecr.aws/arduino/", "influxdb"}

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

func SystemCleanupSoft(ctx context.Context, cfg config.Configuration, staticStore *store.StaticStore, docker command.Cli) (int64, error) {
	totalCleaned := int64(0)

	containersMustStay, err := getRequiredImages(cfg, staticStore)
	if err != nil {
		return totalCleaned, err
	}

	allImages, err := listImagesAlreadyPulled(ctx)
	if err != nil {
		return totalCleaned, err
	}

	imagesToRemove := slices.DeleteFunc(allImages, func(v string) bool {
		return slices.Contains(containersMustStay, v)
	})

	if len(imagesToRemove) == 0 {
		return totalCleaned, nil
	}

	runningApp, err := getRunningApp(ctx, docker.Client())
	if err != nil {
		return totalCleaned, fmt.Errorf("failed to get running app: %w", err)
	}
	if runningApp != nil {
		for item := range StopAndDestroyApp(ctx, *runningApp) {
			if item.GetType() == ErrorType {
				return totalCleaned, item.GetError()
			}
		}
	}

	for _, container := range imagesToRemove {
		imageSize, err := removeImage(ctx, container)
		if err != nil {
			feedback.Printf("Warning: failed to remove image %s - %v", container, err)
			continue
		}
		totalCleaned += imageSize
	}
	return totalCleaned, nil
}

func removeImage(ctx context.Context, imageName string) (int64, error) {
	imageSize, err := getImageSize(imageName, ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get size of image %s: %w", imageName, err)
	}

	cmd, err := paths.NewProcess(nil, "docker", "rmi", "-f", imageName)
	if err != nil {
		return 0, fmt.Errorf("failed to create command to remove docker image %s: %w", imageName, err)
	}

	if err := cmd.RunWithinContext(ctx); err != nil {
		return 0, fmt.Errorf("failed to remove image %s: %v", imageName, err)
	}

	return imageSize, nil
}

func getImageSize(container string, ctx context.Context) (int64, error) {
	cmdImageSize, err := paths.NewProcess(nil, "docker", "image", "inspect", container, "--format", "{{.Size}}")
	if err != nil {
		return 0, err
	}
	containersize, err := cmdImageSize.RunAndCaptureCombinedOutput(ctx)
	if err != nil {
		return 0, err
	}
	trimmedOutput := bytes.TrimSpace(containersize)

	sizeInt64, err := strconv.ParseInt(string(trimmedOutput), 10, 64)
	if err != nil {
		return 0, err
	}

	return sizeInt64, nil
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
