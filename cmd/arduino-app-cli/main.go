package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"strings"

	"github.com/arduino/go-paths-helper"
	dockerClient "github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"go.bug.st/cleanup"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/assets"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
)

// Version will be set a build time with -ldflags
var Version string = "0.0.0-dev"
var RunnerVersion = "0.1.16"

var (
	pythonImage        string
	usedPythonImageTag string
)

func main() {
	docker, err := dockerClient.NewClientWithOpts(
		dockerClient.FromEnv,
		dockerClient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		panic(err)
	}
	defer docker.Close()
	slog.SetLogLoggerLevel(slog.LevelDebug)

	const dockerRegistry = "ghcr.io/bcmi-labs/"
	var dockerPythonImage = fmt.Sprintf("arduino/appslab-python-apps-base:%s", RunnerVersion)
	registryBase := os.Getenv("DOCKER_REGISTRY_BASE")
	if registryBase == "" {
		registryBase = dockerRegistry
	}

	// Python image: image name (repository) and optionally a tag.
	pythonImageAndTag := os.Getenv("DOCKER_PYTHON_BASE_IMAGE")
	if pythonImageAndTag == "" {
		pythonImageAndTag = dockerPythonImage
	}
	pythonImage = path.Join(registryBase, pythonImageAndTag)
	slog.Debug("Using pythonImage", slog.String("image", pythonImage))
	if idx := strings.LastIndex(pythonImage, ":"); idx != -1 {
		usedPythonImageTag = pythonImage[idx+1:]
	}

	composeFolderFS := f.Must(fs.Sub(assets.FS, path.Join("static", RunnerVersion, "compose")))
	brickDocsFS := f.Must(fs.Sub(assets.FS, path.Join("static", RunnerVersion, "docs")))
	assetsFolderFS := f.Must(fs.Sub(assets.FS, path.Join("static", RunnerVersion)))

	var (
		bricksIndex *bricksindex.BricksIndex
		modelsIndex *modelsindex.ModelsIndex
	)

	// In case in local development we use a tag that is not in the index we
	// fallback to dynamicProvisioning
	isUsingDynamicProvision := usedPythonImageTag != RunnerVersion
	provisioner := f.Must(orchestrator.NewProvision(
		docker,
		composeFolderFS,
		isUsingDynamicProvision,
		pythonImage,
	))

	if isUsingDynamicProvision {
		dynamicProvisionDir := paths.TempDir().Join(".cache")
		bricksIndex = f.Must(bricksindex.GenerateBricksIndexFromFile(dynamicProvisionDir))
		modelsIndex = f.Must(modelsindex.GenerateModelsIndexFromFile(dynamicProvisionDir))
	} else {
		bricksIndex = f.Must(bricksindex.GenerateBricksIndex(assetsFolderFS))
		modelsIndex = f.Must(modelsindex.GenerateModelsIndex(assetsFolderFS))
	}

	rootCmd := &cobra.Command{
		Use:   "arduino-app-cli",
		Short: "A CLI to manage the Python app",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.AddCommand(
		newAppCmd(docker, provisioner, modelsIndex, bricksIndex),
		newBrickCmd(brickDocsFS, modelsIndex, bricksIndex),
		newCompletionCommand(),
		newDaemonCmd(docker, provisioner, brickDocsFS, modelsIndex, bricksIndex),
		newPropertiesCmd(),
		newConfigCmd(),
		newSystemCmd(),
		&cobra.Command{
			Use:   "version",
			Short: "Print the version number of Arduino App CLI",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("Arduino App CLI v" + Version)
			},
		},
		newFSCmd(),
	)

	ctx := context.Background()
	ctx, _ = cleanup.InterruptableContext(ctx)
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		slog.Error(err.Error())
	}
}
