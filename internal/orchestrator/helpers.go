package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
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

type AppStatus struct {
	AppPath *paths.Path
	Status  Status
}

func getAppsStatus(ctx context.Context, docker *dockerClient.Client) ([]AppStatus, error) {
	getPythonApp := func() ([]AppStatus, error) {
		containers, err := docker.ContainerList(ctx, container.ListOptions{
			All:     true,
			Filters: filters.NewArgs(filters.Arg("label", DockerAppLabel+"=true")),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list containers: %w", err)
		}
		if len(containers) == 0 {
			return nil, nil
		}

		// We are labeling only the python containr so we assume there is only one container per app.
		apps := make([]AppStatus, 0, len(containers))
		for _, c := range containers {
			appPath, ok := c.Labels[DockerAppPathLabel]
			if !ok {
				return nil, fmt.Errorf("failed to get config files for app %s", c.ID)
			}

			apps = append(apps, AppStatus{
				AppPath: paths.New(appPath),
				Status:  StatusFromDockerState(c.State),
			})
		}

		return apps, nil
	}

	getSketchApp := func() ([]AppStatus, error) {
		// TODO: implement this function
		return nil, nil
	}

	for _, get := range [](func() ([]AppStatus, error)){getPythonApp, getSketchApp} {
		apps, err := get()
		if err != nil {
			return nil, err
		}
		if len(apps) != 0 {
			return apps, nil
		}
	}
	return nil, nil
}

func getAppStatus(ctx context.Context, docker *dockerClient.Client, app app.ArduinoApp) (AppStatus, error) {
	apps, err := getAppsStatus(ctx, docker)
	if err != nil {
		return AppStatus{}, fmt.Errorf("failed to get app status: %w", err)
	}
	idx := slices.IndexFunc(apps, func(a AppStatus) bool {
		return a.AppPath.String() == app.FullPath.String()
	})
	if idx == -1 {
		return AppStatus{}, fmt.Errorf("app %s not found", app.FullPath)
	}
	return apps[idx], nil
}

func getRunningApp(ctx context.Context, docker *dockerClient.Client) (*app.ArduinoApp, error) {
	apps, err := getAppsStatus(ctx, docker)
	if err != nil {
		return nil, fmt.Errorf("failed to get running apps: %w", err)
	}
	idx := slices.IndexFunc(apps, func(a AppStatus) bool {
		return a.Status == "running"
	})
	if idx == -1 {
		return nil, nil
	}
	app, err := app.Load(apps[idx].AppPath.String())
	if err != nil {
		return nil, fmt.Errorf("failed to load running app: %w", err)
	}
	return &app, nil
}

func getAppComposeProjectNameFromApp(app app.ArduinoApp) (string, error) {
	composeProjectName, err := app.FullPath.RelFrom(orchestratorConfig.AppsDir())
	if err != nil {
		return "", fmt.Errorf("failed to get compose project name: %w", err)
	}
	return slug.Make(composeProjectName.String()), nil
}
