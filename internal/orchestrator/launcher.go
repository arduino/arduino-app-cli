// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/moby/moby/api/types/container"
	dockerClient "github.com/moby/moby/client"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/launcher"
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
	// launcherStartTimeout bounds a start request, which stops the running app first.
	launcherStartTimeout = 20 * time.Second
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
// a release runs its app the classic way.
func launcherServed(prj *types.Project) bool {
	main, ok := prj.Services["main"]
	return ok && labeled(main.Labels, DockerAppLauncherLabel)
}

// labeled tells whether the labels carry the flag label.
func labeled(labels map[string]string, label string) bool {
	return labels[label] == "true"
}

// launcherContainer finds the running main container of an app that runs under the app
// launcher, nil when the app has none.
func launcherContainer(containers []container.Summary) *container.Summary {
	for _, c := range containers {
		if labeled(c.Labels, DockerAppMainLabel) && labeled(c.Labels, DockerAppLauncherLabel) && c.State == container.StateRunning {
			return &c
		}
	}
	return nil
}

// launcherClient talks to the launcher of the main container of the app, and to nothing
// else that answers on its socket.
func launcherClient(ctx context.Context, docker dockerClient.APIClient, cfg config.Configuration, appPath *paths.Path, containerID string) *launcher.Client {
	socket := appRuntimeDir(cfg, appPath).Join(launcherSocketFile).String()
	return launcher.New(socket, launcherStartTimeout).WithPeer(launcherCheck(func() (int, error) {
		return containerFirstPID(ctx, docker, containerID)
	}))
}

// launcherCheck accepts the first process of the container only, which run.sh makes the
// launcher. The app is a child of the launcher that shares the socket folder with it, so
// it could listen on the socket path itself.
func launcherCheck(firstPID func() (int, error)) launcher.PeerCheck {
	return func(pid int) error {
		first, err := firstPID()
		if err != nil {
			return err
		}
		if pid != first {
			return fmt.Errorf("pid %d is not the first process of the app container, pid %d", pid, first)
		}
		return nil
	}
}

// containerFirstPID is the first process of a running container, as this host numbers it.
func containerFirstPID(ctx context.Context, docker dockerClient.APIClient, containerID string) (int, error) {
	inspected, err := docker.ContainerInspect(ctx, containerID, dockerClient.ContainerInspectOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to inspect the app container: %w", err)
	}
	state := inspected.Container.State
	if state == nil || !state.Running || state.Pid == 0 {
		return 0, errors.New("the app container is not running")
	}
	return state.Pid, nil
}
