// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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
		for range StopAndDestroyApp(ctx, docker, platform, app) {
			// just consume the iterator
		}
	}

	return app.ProvisioningStateDir().RemoveAll()
}
