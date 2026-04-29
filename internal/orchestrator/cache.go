// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"errors"

	"github.com/docker/cli/cli/command"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

type CleanAppCacheRequest struct {
	ForceClean bool
}

var ErrCleanCacheRunningApp = errors.New("cannot remove cache of a running app")

// CleanAppCache removes the `.cache` folder. If it detects that the app is running
// it tries to stop it first.
func CleanAppCache(
	ctx context.Context,
	docker command.Cli,
	app app.ArduinoApp,
	req CleanAppCacheRequest,
	platform platform.Platform,
) error {
	runningApp, err := getRunningApp(ctx, docker.Client())
	if err != nil {
		return err
	}
	if runningApp != nil && runningApp.FullPath.EqualsTo(app.FullPath) {
		if !req.ForceClean {
			return ErrCleanCacheRunningApp
		}
		// We try to remove docker related resources at best effort
		_ = StopAndDestroyApp(ctx, docker, platform, app, func(StreamMessage) {})
	}

	return app.ProvisioningStateDir().RemoveAll()
}
