package orchestrator

import (
	"context"
	"log/slog"

	dockerClient "github.com/docker/docker/client"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func AppStatus(ctx context.Context, cfg config.Configuration, docker dockerClient.APIClient, idProvider *app.IDProvider) ([]AppInfo, error) {
	appsStatus, err := getAppsStatus(ctx, docker)
	if err != nil {
		return nil, err
	}

	defaultApp, err := GetDefaultApp(cfg)
	if err != nil {
		slog.Warn("unable to get default app", slog.String("error", err.Error()))
	}

	apps := make([]AppInfo, 0, len(appsStatus))
	for _, a := range appsStatus {
		// FIXME: create an helper function to transform an app.ArduinoApp into an ortchestrator.AppInfo

		app, err := app.Load(a.AppPath.String())
		if err != nil {
			slog.Warn("error loading app", "appPath", a.AppPath.String(), "error", err)
			return nil, err
		}

		id, err := idProvider.IDFromPath(a.AppPath)
		if err != nil {
			return nil, err
		}

		isDefault := defaultApp != nil && defaultApp.FullPath.EqualsTo(app.FullPath)

		apps = append(apps, AppInfo{
			ID:          id,
			Name:        app.Descriptor.Name,
			Description: app.Descriptor.Description,
			Icon:        app.Descriptor.Icon,
			Status:      a.Status,
			Example:     id.IsExample(),
			Default:     isDefault,
		})
	}

	return apps, nil
}
