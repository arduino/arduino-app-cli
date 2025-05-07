package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path"
	"time"

	dockerClient "github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"go.bug.st/cleanup"

	"github.com/arduino/arduino-app-cli/internal/api"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/pkg/parser"
)

const DockerRegistry = "ghcr.io/bcmi-labs/"
const DockerPythonImage = "arduino/appslab-python-apps-base:0.0.2"

var pythonImage string

func init() {
	// Registry base: contains the registry and namespace, common to all Arduino docker images.
	registryBase := os.Getenv("DOCKER_REGISTRY_BASE")
	if registryBase == "" {
		registryBase = DockerRegistry
	}

	// Python image: image name (repository) and optionally a tag.
	pythonImageAndTag := os.Getenv("DOCKER_PYTHON_BASE_IMAGE")
	if pythonImageAndTag == "" {
		pythonImageAndTag = DockerPythonImage
	}

	pythonImage = path.Join(registryBase, pythonImageAndTag)
	fmt.Println("Using pythonImage:", pythonImage)
}

func main() {
	docker, err := dockerClient.NewClientWithOpts(
		dockerClient.FromEnv,
		dockerClient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		panic(err)
	}
	defer docker.Close()

	var parsedApp parser.App

	rootCmd := &cobra.Command{
		Use:   "app <APP_PATH>",
		Short: "A CLI to manage the Python app",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if cmd.Name() == "daemon" {
				return
			}
			if len(args) != 1 {
				_ = cmd.Help()
				os.Exit(1)
			}
			app, err := parser.Load(args[0])
			if err != nil {
				log.Panic(err)
			}
			parsedApp = app
		},
	}

	rootCmd.AddCommand(
		&cobra.Command{
			Use:   "stop",
			Short: "Stop the Python app",
			Run: func(cmd *cobra.Command, args []string) {
				stopHandler(cmd.Context(), parsedApp)
			},
		},
		&cobra.Command{
			Use:   "start",
			Short: "Start the Python app",
			Run: func(cmd *cobra.Command, args []string) {
				if parsedApp.MainPythonFile != nil {
					provisionHandler(cmd.Context(), docker, parsedApp)
				}

				startHandler(cmd.Context(), parsedApp)
			},
		},
		&cobra.Command{
			Use:   "logs",
			Short: "Show the logs of the Python app",
			Run: func(cmd *cobra.Command, args []string) {
				logsHandler(cmd.Context(), parsedApp)
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List all running Python apps",
			Run: func(cmd *cobra.Command, args []string) {
				listHandler(cmd.Context())
			},
		},
		&cobra.Command{
			Use:   "provision",
			Short: "Makes sure the Python app deps are downloaded and running",
			Run: func(cmd *cobra.Command, args []string) {
				provisionHandler(cmd.Context(), docker, parsedApp)
			},
		},
		&cobra.Command{
			Use:   "daemon",
			Short: "Run an HTTP server to expose orchestrator functionality thorough REST API",
			Run: func(cmd *cobra.Command, args []string) {
				httpHandler(cmd.Context(), docker)
			},
		},
	)

	ctx := context.Background()
	ctx, cancel := cleanup.InterruptableContext(ctx)
	defer cancel()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		log.Panic(err)
	}
}

func provisionHandler(ctx context.Context, docker *dockerClient.Client, app parser.App) {
	orchestrator.ProvisionApp(ctx, pythonImage, docker, app)
}

func startHandler(ctx context.Context, app parser.App) {
	orchestrator.StartApp(ctx, app)
}

func stopHandler(ctx context.Context, app parser.App) {
	orchestrator.StopApp(ctx, app)
}

func logsHandler(ctx context.Context, app parser.App) {
	orchestrator.AppLogs(ctx, app)
}

func listHandler(ctx context.Context) {
	res, err := orchestrator.ListApps(ctx)
	if err != nil {
		slog.Error(err.Error())
		return
	}
	fmt.Println(string(res.Stdout))
	if len(res.Stderr) > 0 {
		fmt.Println(string(res.Stderr))
	}
}

func httpHandler(ctx context.Context, dockerClient *dockerClient.Client) {
	apiSrv := api.NewHTTPRouter(dockerClient)
	httpSrv := http.Server{
		Addr:              ":8080",
		Handler:           apiSrv,
		ReadHeaderTimeout: 60 * time.Second,
	}
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(err.Error())
		}
	}()

	<-ctx.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	_ = httpSrv.Shutdown(ctx)
	cancel()
}
