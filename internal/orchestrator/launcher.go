// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/types"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

// The main service of an app runs the app launcher, `/run.sh launch`: a warm interpreter that
// starts the app as soon as it is up and ends when the app ends, as the app did. The container
// lives as long as its app, so the app is started, stopped and told apart by the state of its
// container, as an app that runs alone in it. The launcher listens on a socket in the runtime
// folder of the app, through which the running app can be replaced without a new container.
const (
	// launcherSocketDir is where an app container sees the runtime folder of its app.
	launcherSocketDir = "/run/arduino-app-cli"
	// launcherSocketFile is the socket the launcher binds in that folder.
	launcherSocketFile = "app-launcher.sock"
	// maxSocketPathLength is the longest path a Unix socket can be reached by.
	maxSocketPathLength = 107
)

// appRuntimeDir is the folder of the app on this board the container sees at launcherSocketDir.
// It is named after the project, cut where needed to keep the socket in it within reach.
func appRuntimeDir(cfg config.Configuration, appPath *paths.Path) *paths.Path {
	dir := cfg.RuntimeDir()
	room := maxSocketPathLength - len(dir.Join(launcherSocketFile).String()) - len("/")
	return dir.Join(runtimeKey(composeProjectName(appPath, cfg.AppsDir()), room))
}

// runtimeKey is the project name, cut to the limit and told apart by a hash when longer. A
// limit too short for a name leaves the hash alone.
func runtimeKey(project string, limit int) string {
	if len(project) <= limit {
		return project
	}
	sum := sha256.Sum256([]byte(project))
	suffix := hex.EncodeToString(sum[:6])
	if limit <= len(suffix)+1 {
		return suffix
	}
	return strings.TrimRight(project[:limit-len(suffix)-1], "-") + "-" + suffix
}

// launcherServed tells whether the project runs its main service through the app launcher:
// a release frozen before the launcher runs its app the classic way.
func launcherServed(prj *types.Project) bool {
	main, ok := prj.Services["main"]
	return ok && main.Labels[DockerAppLauncherLabel] == "true"
}
