// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/arduino/go-paths-helper"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dockerClient "github.com/docker/docker/client"
	"github.com/gosimple/slug"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

type AppStatusInfo struct {
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
func parseAppStatus(containers []container.Summary) []AppStatusInfo {
	apps := make([]AppStatusInfo, 0, len(containers))
	appsStatusMap := make(map[string][]Status)
	for _, c := range containers {
		appPath, ok := c.Labels[DockerAppPathLabel]
		if !ok {
			continue
		}
		appsStatusMap[appPath] = append(appsStatusMap[appPath], StatusFromDockerState(c.State, c.Status))

	}

	appendResult := func(appPath *paths.Path, status Status) {
		apps = append(apps, AppStatusInfo{
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
) ([]AppStatusInfo, error) {
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

func getAppStatus(
	ctx context.Context,
	docker dockerClient.APIClient,
	app app.ArduinoApp,
) (AppStatusInfo, error) {
	containers, err := docker.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", DockerAppPathLabel+"="+app.FullPath.String())),
	})
	if err != nil {
		return AppStatusInfo{}, fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		return AppStatusInfo{
			AppPath: app.FullPath,
			Status:  StatusUninitialized,
		}, nil
	}

	appInfo := parseAppStatus(containers)
	if len(appInfo) == 0 {
		return AppStatusInfo{}, fmt.Errorf("no app status found for app at path %s", app.FullPath)
	}
	return appInfo[0], nil
}

func getRunningApp(
	ctx context.Context,
	docker dockerClient.APIClient,
) (*app.ArduinoApp, error) {
	apps, err := getAppsStatus(ctx, docker)
	if err != nil {
		return nil, fmt.Errorf("failed to get running apps: %w", err)
	}
	idx := slices.IndexFunc(apps, func(a AppStatusInfo) bool {
		return a.Status == StatusRunning || a.Status == StatusStarting
	})
	if idx == -1 {
		return nil, nil
	}
	app, err := app.Load(apps[idx].AppPath)
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

func GetCustomErrorFomDockerEvent(message string) error {
	if strings.HasSuffix(message, ": unauthorized") {
		return errors.New("could not reach the Docker registry to download base image. Please make sure to be authorized to download from it or flash the board with the latest Arduino Linux image. Details: " + message + ")")
	}

	if strings.HasSuffix(message, ": connection refused") || strings.Contains(message, ": no such host") {
		return errors.New("could not reach the Docker registry to download base image. Please check your internet connection or flash the board with the latest Arduino Linux image. Details: " + message + ")")
	}

	return nil
}

type ledTarget string

const (
	ledTriggerTarget    ledTarget = "trigger"
	ledBrightnessTarget ledTarget = "brightness"
)

const (
	noneTrigger    string = "none"
	defaultTrigger string = "default"
)

func setLeds(ledPath *paths.Path, target ledTarget, value string) error {
	ledPath = ledPath.Join(string(target))
	if !ledPath.Exist() {
		return fmt.Errorf("LED path %s does not exist", ledPath)
	}
	if err := ledPath.WriteFile([]byte(value)); err != nil {
		return fmt.Errorf("failed to set LED %s to %s: %w", ledPath, target, err)
	}
	return nil
}

func setLedsToUserControlledMode(platform platform.Platform) error {
	for _, led := range platform.Linux.BoardLeds {
		if err := setLeds(led, ledTriggerTarget, noneTrigger); err != nil {
			return err
		}
	}
	return nil
}

func restoreLedsState(platform platform.Platform) error {
	for _, led := range platform.Linux.BoardLeds {
		if err := setLeds(led, ledBrightnessTarget, string("0")); err != nil {
			return err
		}
		if err := setLeds(led, ledTriggerTarget, defaultTrigger); err != nil {
			return err
		}
	}
	return nil
}

func SetArduinoCliConfig(ctx context.Context, cli rpc.ArduinoCoreServiceServer) error {
	if _, err := cli.SettingsSetValue(ctx, &rpc.SettingsSetValueRequest{
		Key:          "network.connection_timeout",
		EncodedValue: "600s",
		ValueFormat:  "cli",
	}); err != nil {
		return err
	}

	// Set the data dir if specified via the ARDUINO_DIRECTORIES_DATA env var
	if dataDir, ok := os.LookupEnv("ARDUINO_DIRECTORIES_DATA"); ok {
		_, err := cli.SettingsSetValue(ctx, &rpc.SettingsSetValueRequest{
			Key:          "directories.data",
			EncodedValue: dataDir,
			ValueFormat:  "cli",
		})
		if err != nil {
			return fmt.Errorf("could not set data directory: %w", err)
		}
	}

	// Set the additional urls via ARDUINO_BOARD_MANAGER_ADDITIONAL_URLS env var
	if urls, ok := os.LookupEnv("ARDUINO_BOARD_MANAGER_ADDITIONAL_URLS"); ok {
		_, err := cli.SettingsSetValue(ctx, &rpc.SettingsSetValueRequest{
			Key:          "board_manager.additional_urls",
			EncodedValue: urls,
			ValueFormat:  "cli",
		})
		if err != nil {
			return fmt.Errorf("could not set additional urls: %w", err)
		}
	}

	return nil
}
