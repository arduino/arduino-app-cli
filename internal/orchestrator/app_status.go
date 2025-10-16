// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package orchestrator

import (
	"context"
	"fmt"
	"iter"
	"log/slog"

	"github.com/docker/cli/cli/command"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func AppStatusEvents(ctx context.Context, cfg config.Configuration, docker command.Cli, idProvider *app.IDProvider) iter.Seq2[AppInfo, error] {
	chanMsg, chanError := docker.Client().Events(ctx, events.ListOptions{
		Filters: filters.NewArgs(
			filters.Arg("label", DockerAppLabel+"=true"),
			filters.Arg("type", string(events.ContainerEventType)),
			filters.Arg("event", "create"),
			filters.Arg("event", "start"),
			filters.Arg("event", "stop"),
			filters.Arg("event", "die"),
			filters.Arg("event", "restart"),
			filters.Arg("event", "destroy"),
			filters.Arg("event", "delete"),
		),
	})

	return func(yield func(AppInfo, error) bool) {
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
					_ = yield(AppInfo{}, fmt.Errorf("error listening to docker events: %w", err))
					return
				}
			case event := <-chanMsg:
				appStatus, err := parseDockerStatusEvent(ctx, cfg, docker, idProvider, event)
				if err != nil {
					slog.Error("Unable to get apps status", slog.String("error", err.Error()))
					if !yield(AppInfo{}, err) {
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

func parseDockerStatusEvent(ctx context.Context, cfg config.Configuration, docker command.Cli, idProvider *app.IDProvider, event events.Message) (AppInfo, error) {

	if pathLabel, ok := event.Actor.Attributes[DockerAppPathLabel]; ok {

		appStatus, err := getAppStatusByPath(ctx, docker.Client(), pathLabel)
		if err != nil {
			return AppInfo{}, err
		}

		if appStatus == nil {
			return AppInfo{}, fmt.Errorf("app containers not found for: %s", pathLabel)
		}

		defaultApp, err := GetDefaultApp(cfg)
		if err != nil {
			slog.Warn("unable to get default app", slog.String("error", err.Error()))
		}

		// FIXME: create an helper function to transform an app.ArduinoApp into an ortchestrator.AppInfo
		app, err := app.Load(appStatus.AppPath.String())
		if err != nil {
			slog.Warn("error loading app", "appPath", appStatus.AppPath.String(), "error", err)
			return AppInfo{}, err
		}

		id, err := idProvider.IDFromPath(appStatus.AppPath)
		if err != nil {
			return AppInfo{}, err
		}

		isDefault := defaultApp != nil && defaultApp.FullPath.EqualsTo(app.FullPath)

		return AppInfo{
			ID:          id,
			Name:        app.Descriptor.Name,
			Description: app.Descriptor.Description,
			Icon:        app.Descriptor.Icon,
			Status:      appStatus.Status,
			Example:     id.IsExample(),
			Default:     isDefault,
		}, nil

	}
	return AppInfo{}, fmt.Errorf("unable to find app path label in event")

}
