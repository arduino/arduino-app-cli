// The servicelocator pkg should be used only under cmd/arduino-app-cli as a convenience to build our DI.

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
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricks"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/store"
)

var globalConfig config.Configuration

func Init(cfg config.Configuration) {
	globalConfig = cfg
}

var (
	// Do not manually modify this, we keep it updated with the `task generate:bricks-and-models-index`
	runnerVersion = "0.1.16"

	GetBricksIndex = sync.OnceValue(func() *bricksindex.BricksIndex {
		return f.Must(bricksindex.GenerateBricksIndexFromFile(GetStaticStore().GetAssetsFolder()))
	})

	GetModelsIndex = sync.OnceValue(func() *modelsindex.ModelsIndex {
		return f.Must(modelsindex.GenerateModelsIndexFromFile(GetStaticStore().GetAssetsFolder()))
	})

	GetProvisioner = sync.OnceValue(func() *orchestrator.Provision {
		pythonImage, usedPythonImageTag := getPythonImageAndTag()
		slog.Debug("Using pythonImage", slog.String("image", pythonImage))

		return f.Must(orchestrator.NewProvision(
			GetDockerClient(),
			pythonImage,
			usedPythonImageTag,
			runnerVersion,
			globalConfig,
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
		return store.NewStaticStore(globalConfig.AssetsDir().Join(GetUsedPythonImageTag()).String())
	})

	GetBrickService = sync.OnceValue(func() *bricks.Service {
		return bricks.NewService(
			GetModelsIndex(),
			GetBricksIndex(),
			GetStaticStore(),
		)
	})

	GetAppIDProvider = sync.OnceValue(func() *app.IDProvider {
		return app.NewAppIDProvider(globalConfig)
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
		pythonImageAndTag = fmt.Sprintf("arduino/appslab-python-apps-base:%s", runnerVersion)
	}
	pythonImage := path.Join(registryBase, pythonImageAndTag)
	var usedPythonImageTag string
	if idx := strings.LastIndex(pythonImage, ":"); idx != -1 {
		usedPythonImageTag = pythonImage[idx+1:]
	}
	return pythonImage, usedPythonImageTag
}
