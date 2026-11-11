// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"fmt"
	"iter"
	"log/slog"

	"github.com/arduino/go-paths-helper"
	"github.com/docker/cli/cli/command"
	"github.com/moby/moby/api/types/events"

	"github.com/arduino/arduino-app-cli/internal/appbook"
	"github.com/arduino/arduino-app-cli/internal/dockerhelper"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func AppStatusEvents(ctx context.Context, cfg config.Configuration, docker command.Cli, idProvider *appid.Provider) iter.Seq2[appbook.AppInfo, error] {
	chanMsg, chanError := dockerhelper.ContainerEvents(ctx, docker.Client(), DockerAppLabel+"=true",
		"create", "start", "stop", "die", "restart", "destroy", "delete")

	return func(yield func(appbook.AppInfo, error) bool) {
		for {
			select {
			case <-ctx.Done():
				slog.Debug("Stopping to listen to docker events")
				return
			default:
			}

			select {

			case err := <-chanError:
				if err != nil {
					slog.Error("Error listening to docker events", slog.String("error", err.Error()))
					_ = yield(appbook.AppInfo{}, fmt.Errorf("error listening to docker events: %w", err))
					return
				}
			case event := <-chanMsg:
				appStatus, err := parseDockerStatusEvent(ctx, cfg, docker, idProvider, event)
				if err != nil {
					slog.Error("Unable to get apps status", slog.String("error", err.Error()))
					if !yield(appbook.AppInfo{}, err) {
						return
					}
				}
				if !yield(appStatus, nil) {
					return
				}
			}

		}
	}
}

func parseDockerStatusEvent(ctx context.Context, cfg config.Configuration, docker command.Cli, idProvider *appid.Provider, event events.Message) (appbook.AppInfo, error) {

	if pathLabel, ok := event.Actor.Attributes[DockerAppPathLabel]; ok {
		app, err := app.Load(paths.New(pathLabel))
		if err != nil {
			slog.Warn("error loading app", "appPath", pathLabel, "error", err)
			return appbook.AppInfo{}, err
		}

		appStatus, err := getAppStatus(ctx, docker.Client(), app)
		if err != nil {
			return appbook.AppInfo{}, err
		}

		defaultApp, err := appbook.GetDefaultApp(cfg)
		if err != nil {
			slog.Warn("unable to get default app", slog.String("error", err.Error()))
		}

		// FIXME: create an helper function to transform an app.ArduinoApp into an ortchestrator.AppInfo
		id, err := idProvider.IDFromPath(appStatus.AppPath)
		if err != nil {
			return appbook.AppInfo{}, err
		}

		isDefault := defaultApp != nil && defaultApp.FullPath.EqualsTo(app.FullPath)

		return appbook.AppInfo{
			ID:          id,
			Name:        app.Descriptor.Name,
			Description: app.Descriptor.Description,
			Icon:        app.Descriptor.Icon,
			Status:      appStatus.Status,
			Example:     id.IsExample(),
			Default:     isDefault,
		}, nil

	}
	return appbook.AppInfo{}, fmt.Errorf("unable to find app path label in event")

}
