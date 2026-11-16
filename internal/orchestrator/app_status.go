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
	"github.com/moby/moby/api/types/container"

	"github.com/arduino/arduino-app-cli/internal/dockerhelper"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

// AppStatusEvents yields the status of an app each time one of its containers changes, as the
// docker events tell, and each time the launcher of a running main container tells its app
// started or ended: an app that ends in a container that stays up raises no docker event.
func AppStatusEvents(ctx context.Context, cfg config.Configuration, docker command.Cli, idProvider *appid.Provider) iter.Seq2[AppInfo, error] {
	chanMsg, chanError := dockerhelper.ContainerEvents(ctx, docker.Client(), DockerAppLabel+"=true",
		"create", "start", "stop", "die", "restart", "destroy", "delete")

	return func(yield func(AppInfo, error) bool) {
		// The launchers are followed as long as the events are read
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		launchers := newLauncherWatch(docker.Client())

		// The apps that run now are followed from the start, not from their next docker event
		containers, err := dockerhelper.Containers(ctx, docker.Client(), DockerAppLabel+"=true")
		if err != nil {
			slog.Warn("Unable to get apps status", slog.String("error", err.Error()))
		}
		launchers.sync(ctx, containers)
		last := statusByPath(containers, launchers.statuses())

		for {
			var changed []AppStatusInfo
			select {
			case <-ctx.Done():
				slog.Debug("Stopping to listen to docker events")
				return
			case err := <-chanError:
				if err != nil {
					slog.Error("Error listening to docker events", slog.String("error", err.Error()))
					_ = yield(AppInfo{}, fmt.Errorf("error listening to docker events: %w", err))
					return
				}
			case event := <-chanMsg:
				listed, err := dockerhelper.Containers(ctx, docker.Client(), DockerAppLabel+"=true")
				if err != nil {
					slog.Error("Unable to get apps status", slog.String("error", err.Error()))
					if !yield(AppInfo{}, err) {
						return
					}
					continue
				}
				containers = listed
				launchers.sync(ctx, containers)
				current := statusByPath(containers, launchers.statuses())
				// The app of the event is reported whatever its status, as every event was
				if pathLabel, ok := event.Actor.Attributes[DockerAppPathLabel]; ok {
					status, known := current[pathLabel]
					if !known {
						status = StatusUninitialized
					}
					changed = append(changed, AppStatusInfo{AppPath: paths.New(pathLabel), Status: status})
				}
				last = current
			case update := <-launchers.updates:
				if !launchers.apply(update) {
					continue
				}
				current := statusByPath(containers, launchers.statuses())
				for appPath, status := range current {
					if last[appPath] != status {
						changed = append(changed, AppStatusInfo{AppPath: paths.New(appPath), Status: status})
					}
				}
				last = current
			}

			for _, change := range changed {
				info, err := appInfoFromStatus(cfg, idProvider, change)
				if err != nil {
					slog.Error("Unable to get apps status", slog.String("error", err.Error()))
					if !yield(AppInfo{}, err) {
						return
					}
					continue
				}
				if !yield(info, nil) {
					return
				}
			}
		}
	}
}

func statusByPath(containers []container.Summary, appStatuses map[string]Status) map[string]Status {
	statuses := parseAppStatus(containers, appStatuses)
	result := make(map[string]Status, len(statuses))
	for _, s := range statuses {
		result[s.AppPath.String()] = s.Status
	}
	return result
}

func appInfoFromStatus(cfg config.Configuration, idProvider *appid.Provider, status AppStatusInfo) (AppInfo, error) {
	app, err := app.Load(status.AppPath)
	if err != nil {
		slog.Warn("error loading app", "appPath", status.AppPath.String(), "error", err)
		return AppInfo{}, err
	}

	defaultApp, err := GetDefaultApp(cfg)
	if err != nil {
		slog.Warn("unable to get default app", slog.String("error", err.Error()))
	}

	// FIXME: create an helper function to transform an app.ArduinoApp into an ortchestrator.AppInfo
	id, err := idProvider.IDFromPath(status.AppPath)
	if err != nil {
		return AppInfo{}, err
	}

	isDefault := defaultApp != nil && defaultApp.FullPath.EqualsTo(app.FullPath)

	return AppInfo{
		ID:          id,
		Name:        app.Descriptor.Name,
		Description: app.Descriptor.Description,
		Icon:        app.Descriptor.Icon,
		Status:      status.Status,
		Example:     id.IsExample(),
		Default:     isDefault,
	}, nil
}
