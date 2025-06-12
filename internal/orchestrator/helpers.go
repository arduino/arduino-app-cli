package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/arduino/go-paths-helper"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dockerClient "github.com/docker/docker/client"
	"github.com/gosimple/slug"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

func dockerComposeListServices(ctx context.Context, composeFile *paths.Path) ([]string, error) {
	process, err := paths.NewProcess(nil, "docker", "compose", "-f", composeFile.String(), "config", "--services")
	if err != nil {
		return nil, err
	}
	stdout, stderr, err := process.RunAndCaptureOutput(ctx)
	if len(stderr) > 0 {
		slog.Error("docker compose config error", slog.String("stderr", string(stderr)))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to run docker compose config: %w", err)
	}

	if len(stdout) == 0 {
		return nil, nil
	}

	return strings.Split(strings.TrimSpace(string(stdout)), "\n"), nil
}

type DockerComposeAppStatusResponse struct {
	Name   string `json:"Name"`
	Status string `json:"Status"`
}

func dockerComposeAppStatus(ctx context.Context, app app.ArduinoApp) (DockerComposeAppStatusResponse, error) {
	mainCompose, err := getProvisioningStateDir(app)
	if err != nil {
		return DockerComposeAppStatusResponse{}, err
	}
	composeProjectName, err := getAppComposeProjectNameFromApp(app)
	if err != nil {
		return DockerComposeAppStatusResponse{}, err
	}

	process, err := paths.NewProcess(nil, "docker", "compose", "-f", mainCompose.String(), "ls", "--format", "json", "--all", "--filter", fmt.Sprintf("name=%s", composeProjectName))
	if err != nil {
		return DockerComposeAppStatusResponse{}, err
	}
	stdout, stderr, err := process.RunAndCaptureOutput(ctx)
	if len(stderr) > 0 {
		slog.Error("docker compose config error", slog.String("stderr", string(stderr)))
	}
	if err != nil {
		return DockerComposeAppStatusResponse{}, fmt.Errorf("failed to run docker compose config: %w", err)
	}

	var statusResponse []DockerComposeAppStatusResponse
	if err := json.Unmarshal(stdout, &statusResponse); err != nil {
		return DockerComposeAppStatusResponse{}, fmt.Errorf("failed to unmarshal docker compose status response: %w", err)
	}

	if len(statusResponse) == 0 {
		return DockerComposeAppStatusResponse{}, fmt.Errorf("failed to find app status in docker compose response")
	}
	// It is possible that the --filter returns multiple items as it's not an exact match
	var match DockerComposeAppStatusResponse
	for _, v := range statusResponse {
		if v.Name == composeProjectName {
			match = v
			break
		}
	}

	if match == (DockerComposeAppStatusResponse{}) {
		return DockerComposeAppStatusResponse{}, fmt.Errorf("failed to find app status in docker compose response")
	}

	// The response from compose is in the form of "state(number_services)". Example: "running(2)"
	// We only want the state, so we remove the number of services
	idx := strings.Index(match.Status, "(")
	if idx != -1 {
		match.Status = match.Status[:idx]
	}

	return match, nil
}

func getRunningApp(ctx context.Context, docker *dockerClient.Client) (*app.ArduinoApp, error) {
	getPythonApp := func() (*app.ArduinoApp, error) {
		containers, err := docker.ContainerList(ctx, container.ListOptions{
			Filters: filters.NewArgs(filters.Arg("label", DockerAppLabel+"=true")),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list containers: %w", err)
		}
		if len(containers) > 1 {
			return nil, fmt.Errorf("multiple running apps found: %d", len(containers))
		}
		if len(containers) == 0 {
			return nil, nil
		}

		container := containers[0]
		inspect, err := docker.ContainerInspect(ctx, container.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect container %s: %w", container.ID, err)
		}
		appPath, ok := inspect.Config.Labels[DockerAppPathLabel]
		if !ok {
			return nil, fmt.Errorf("failed to get config files for app %s", container.ID)
		}

		app, err := app.Load(appPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load app %s: %w", appPath, err)
		}
		return &app, nil
	}

	getSketchApp := func() (*app.ArduinoApp, error) {
		// TODO: implement this function
		return nil, nil
	}

	for _, get := range [](func() (*app.ArduinoApp, error)){getPythonApp, getSketchApp} {
		app, err := get()
		if err != nil {
			return nil, err
		}
		if app != nil {
			return app, nil
		}
	}
	return nil, nil
}

func getAppComposeProjectNameFromApp(app app.ArduinoApp) (string, error) {
	composeProjectName, err := app.FullPath.RelFrom(orchestratorConfig.AppsDir())
	if err != nil {
		return "", fmt.Errorf("failed to get compose project name: %w", err)
	}
	return slug.Make(composeProjectName.String()), nil
}
