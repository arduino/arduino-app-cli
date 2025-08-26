package orchestrator

import (
	"context"
	"fmt"
	"slices"

	"github.com/arduino/go-paths-helper"
	"github.com/docker/cli/cli/command"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dockerClient "github.com/docker/docker/client"
	"github.com/gosimple/slug"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

type AppStatus struct {
	AppPath *paths.Path
	Status  Status
}

// parseAppStatus takes all the containers that matches the DockerAppLabel,
// and construct a map of the state of an app and all its dependencies state.
// For app that have at least 1 dependency, we calculate the overall state
// as follow:
//
//	running: all running
//	stopped: all stopped
//	failed: at least one failed
//	stopping: at least one stopping
//	starting: at least one starting
func parseAppStatus(containers []container.Summary) []AppStatus {
	apps := make([]AppStatus, 0, len(containers))
	appsStatusMap := make(map[string][]Status)
	for _, c := range containers {
		appPath, ok := c.Labels[DockerAppPathLabel]
		if !ok {
			continue
		}
		appsStatusMap[appPath] = append(appsStatusMap[appPath], StatusFromDockerState(c.State))
	}

	appendResult := func(appPath *paths.Path, status Status) {
		apps = append(apps, AppStatus{
			AppPath: appPath,
			Status:  status,
		})
	}

	for appPath, s := range appsStatusMap {
		f.Assert(len(s) != 0, "status slice is zero")

		appPath := paths.New(appPath)

		//	running: all running
		if !slices.ContainsFunc(s, func(v Status) bool { return v != StatusRunning }) {
			appendResult(appPath, StatusRunning)
			continue
		}
		//	stopped: all stopped
		if !slices.ContainsFunc(s, func(v Status) bool { return v != StatusStopped }) {
			appendResult(appPath, StatusStopped)
			continue
		}

		// ...else we have multiple different status we calculate the status
		// among the possible left: {failed, stopping, starting}
		if slices.ContainsFunc(s, func(v Status) bool { return v == StatusFailed }) {
			appendResult(appPath, StatusFailed)
			continue
		}
		if slices.ContainsFunc(s, func(v Status) bool { return v == StatusStopping }) {
			appendResult(appPath, StatusStopping)
			continue
		}
		if slices.ContainsFunc(s, func(v Status) bool { return v == StatusStarting }) {
			appendResult(appPath, StatusStarting)
			continue
		}
	}

	return apps
}

func getAppsStatus(
	ctx context.Context,
	docker dockerClient.APIClient,
) ([]AppStatus, error) {
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
		return parseAppStatus(containers), nil
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

func getAppStatus(
	ctx context.Context,
	docker command.Cli,
	app app.ArduinoApp,
) (AppStatus, error) {
	apps, err := getAppsStatus(ctx, docker.Client())
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

func getRunningApp(
	ctx context.Context,
	docker dockerClient.APIClient,
) (*app.ArduinoApp, error) {
	apps, err := getAppsStatus(ctx, docker)
	if err != nil {
		return nil, fmt.Errorf("failed to get running apps: %w", err)
	}
	idx := slices.IndexFunc(apps, func(a AppStatus) bool {
		return a.Status == StatusRunning || a.Status == StatusStarting
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

func getAppComposeProjectNameFromApp(app app.ArduinoApp, cfg config.Configuration) (string, error) {
	composeProjectName, err := app.FullPath.RelFrom(cfg.AppsDir())
	if err != nil {
		return "", fmt.Errorf("failed to get compose project name: %w", err)
	}
	return slug.Make(composeProjectName.String()), nil
}

func findAppPathByName(name string, cfg config.Configuration) (*paths.Path, bool) {
	appFolderName := slug.Make(name)
	basePath := cfg.AppsDir().Join(appFolderName)
	return basePath, basePath.Exist()
}
