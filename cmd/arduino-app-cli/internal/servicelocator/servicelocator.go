package servicelocator

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"
	"sync"

	dockerCommand "github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	dockerClient "github.com/docker/docker/client"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricks"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/store"
)

var (
	// Do not manually modify this, we keep it updated with the `task generate:bricks-and-models-index`
	RunnerVersion = "0.1.16"

	GetBricksIndex = sync.OnceValue(func() *bricksindex.BricksIndex {
		var bIndex *bricksindex.BricksIndex
		if GetProvisioner().IsUsingDynamicProvision() {
			dynamicProvisionDir := GetProvisioner().DynamicProvisionDir()
			bIndex = f.Must(bricksindex.GenerateBricksIndexFromFile(dynamicProvisionDir))
		} else {
			bricksIndexContent := f.Must(GetStaticStore().GetBricksListFile())
			defer bricksIndexContent.Close()
			bIndex = f.Must(bricksindex.LoadBricksIndex(bricksIndexContent))
		}
		return bIndex
	})

	GetModelsIndex = sync.OnceValue(func() *modelsindex.ModelsIndex {
		var mIndex *modelsindex.ModelsIndex
		if GetProvisioner().IsUsingDynamicProvision() {
			dynamicProvisionDir := GetProvisioner().DynamicProvisionDir()
			mIndex = f.Must(modelsindex.GenerateModelsIndexFromFile(dynamicProvisionDir))
		} else {
			modelsIndexContent := f.Must(GetStaticStore().GetModelsListFile())
			defer modelsIndexContent.Close()
			mIndex = f.Must(modelsindex.LoadModelsIndex(modelsIndexContent))
		}
		return mIndex
	})

	GetProvisioner = sync.OnceValue(func() *orchestrator.Provision {
		pythonImage, usedPythonImageTag := getPythonImageAndTag()
		slog.Debug("Using pythonImage", slog.String("image", pythonImage))

		return f.Must(orchestrator.NewProvision(
			GetDockerClient(),
			GetStaticStore(),
			usedPythonImageTag != RunnerVersion,
			pythonImage,
		))
	})

	docker *dockerCommand.DockerCli

	GetDockerClient = sync.OnceValue(func() *dockerCommand.DockerCli {
		docker = f.Must(dockerCommand.NewDockerCli(
			dockerCommand.WithAPIClient(
				f.Must(dockerClient.NewClientWithOpts(
					dockerClient.FromEnv,
					dockerClient.WithAPIVersionNegotiation(),
				)),
			),
		))
		if err := docker.Initialize(flags.NewClientOptions()); err != nil {
			panic(err)
		}
		return docker
	})

	CloseDockerClient = func() error {
		if docker != nil {
			return docker.Client().Close()
		}
		return nil
	}

	GetUsedPythonImageTag = sync.OnceValue(func() string {
		_, usedPythonImageTag := getPythonImageAndTag()
		return usedPythonImageTag
	})

	GetStaticStore = sync.OnceValue(func() *store.StaticStore {
		return store.NewStaticStore(RunnerVersion)
	})

	GetBrickService = sync.OnceValue(func() *bricks.Service {
		return bricks.NewService(
			GetModelsIndex(),
			GetBricksIndex(),
			GetStaticStore(),
		)
	})
)

func getPythonImageAndTag() (string, string) {
	registryBase := os.Getenv("DOCKER_REGISTRY_BASE")
	if registryBase == "" {
		registryBase = "ghcr.io/bcmi-labs/"
	}

	// Python image: image name (repository) and optionally a tag.
	pythonImageAndTag := os.Getenv("DOCKER_PYTHON_BASE_IMAGE")
	if pythonImageAndTag == "" {
		pythonImageAndTag = fmt.Sprintf("arduino/appslab-python-apps-base:%s", RunnerVersion)
	}
	pythonImage := path.Join(registryBase, pythonImageAndTag)
	var usedPythonImageTag string
	if idx := strings.LastIndex(pythonImage, ":"); idx != -1 {
		usedPythonImageTag = pythonImage[idx+1:]
	}
	return pythonImage, usedPythonImageTag
}
