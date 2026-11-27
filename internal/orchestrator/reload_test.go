// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/moby/moby/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/launcher"
)

func serviceContainer(project, service string, state container.ContainerState, labels map[string]string) container.Summary {
	all := map[string]string{composeProjectLabel: project, composeServiceLabel: service}
	for name, value := range labels {
		all[name] = value
	}
	return container.Summary{ID: project + "-" + service, State: state, Labels: all}
}

func TestAProjectIsUpWhenEachOfItsServicesHasARunningContainer(t *testing.T) {
	prj := &types.Project{Name: "my-app", Services: types.Services{"main": {}, "dbstorage-influx": {}}}
	main := serviceContainer("my-app", "main", container.StateRunning, nil)

	assert.True(t, projectServicesRunning(prj, []container.Summary{main, serviceContainer("my-app", "dbstorage-influx", container.StateRunning, nil)}))
	assert.False(t, projectServicesRunning(prj, []container.Summary{main}), "a service has no container")
	assert.False(t, projectServicesRunning(prj, []container.Summary{main, serviceContainer("my-app", "dbstorage-influx", container.StateExited, nil)}), "a service has stopped")
	assert.False(t, projectServicesRunning(prj, []container.Summary{main, serviceContainer("other-app", "dbstorage-influx", container.StateRunning, nil)}), "the container of another project does not count")
	assert.False(t, projectServicesRunning(&types.Project{Name: "my-app"}, []container.Summary{main}), "a project without services runs nothing")
}

func TestTheLauncherContainerIsTheRunningMainContainerUnderTheLauncher(t *testing.T) {
	launched := map[string]string{DockerAppMainLabel: "true", DockerAppLauncherLabel: "true"}
	sidecar := serviceContainer("my-app", "dbstorage-influx", container.StateRunning, nil)

	found := launcherContainer([]container.Summary{sidecar, serviceContainer("my-app", "main", container.StateRunning, launched)})
	require.NotNil(t, found)
	assert.Equal(t, "my-app-main", found.ID)

	assert.Nil(t, launcherContainer([]container.Summary{sidecar, serviceContainer("my-app", "main", container.StateExited, launched)}), "a stopped container answers nothing")
	classic := map[string]string{DockerAppMainLabel: "true"}
	assert.Nil(t, launcherContainer([]container.Summary{sidecar, serviceContainer("my-app", "main", container.StateRunning, classic)}), "a release runs its app without the launcher")
}

func TestOnlyTheFirstProcessOfTheContainerIsItsLauncher(t *testing.T) {
	check := launcherCheck(func() (int, error) { return 4242, nil })
	assert.NoError(t, check(4242))
	assert.Error(t, check(4250), "the app is a child of the launcher")

	gone := errors.New("the app container is not running")
	assert.ErrorIs(t, launcherCheck(func() (int, error) { return 0, gone })(4242), gone)
}

func TestARestartRecoversFromAStartTheLauncherDidNotDo(t *testing.T) {
	// macOS limits socket paths to 104 bytes: t.TempDir() is often longer
	dir, err := os.MkdirTemp("", "l")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	start := func(socket string) error {
		_, err := launcher.New(socket, time.Second).Start(t.Context(), nil)
		return err
	}

	_, restart := launcherRefusal(nil)
	assert.False(t, restart, "the app was reloaded")

	for _, code := range []string{launcher.CodeStale, launcher.CodeBusy} {
		reason, restart := launcherRefusal(&launcher.Error{Code: code})
		assert.True(t, restart, code)
		assert.NotEmpty(t, reason, code)
	}

	_, restart = launcherRefusal(start(filepath.Join(dir, "missing.sock")))
	assert.True(t, restart, "the container did not reach its launcher yet")

	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(dir, "left.sock"), Net: "unix"})
	require.NoError(t, err)
	listener.SetUnlinkOnClose(false)
	require.NoError(t, listener.Close())
	_, restart = launcherRefusal(start(filepath.Join(dir, "left.sock")))
	assert.True(t, restart, "a socket nothing listens on")

	for _, failure := range []error{&launcher.Error{Code: launcher.CodeInternal}, launcher.ErrPeer, errors.New("no answer from the app launcher to start: i/o timeout")} {
		_, restart := launcherRefusal(failure)
		assert.False(t, restart, failure.Error())
	}
}
