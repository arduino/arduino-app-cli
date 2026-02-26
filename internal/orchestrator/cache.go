// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
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
