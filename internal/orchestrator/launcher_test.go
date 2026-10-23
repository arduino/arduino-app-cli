// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"strings"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/moby/moby/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeKeyFitsASocketPath(t *testing.T) {
	assert.Equal(t, "my-app", runtimeKey("my-app"))
	long := strings.Repeat("a-very-long-app-name-", 5)
	key := runtimeKey(long)
	assert.LessOrEqual(t, len(key), maxRuntimeKeyLength)
	assert.True(t, strings.HasPrefix(key, "a-very-long-app-name-"))
	assert.NotEqual(t, key, runtimeKey(long+"x"), "two long names are told apart")
}

func TestLauncherServedReadsTheMainServiceLabel(t *testing.T) {
	served := &types.Project{Services: types.Services{mainServiceName: {Labels: types.Labels{DockerAppLauncherLabel: "true"}}}}
	assert.True(t, launcherServed(served))
	classic := &types.Project{Services: types.Services{mainServiceName: {Labels: types.Labels{DockerAppMainLabel: "true"}}}}
	assert.False(t, launcherServed(classic), "a release frozen before the launcher runs its app the classic way")
	assert.False(t, launcherServed(&types.Project{}))
}

func TestTheLauncherDecidesTheStatusOfTheAppItDrives(t *testing.T) {
	appPath := paths.New("/apps/one").String()
	main := container.Summary{ID: "main", State: container.StateRunning, Labels: map[string]string{
		DockerAppPathLabel: appPath, DockerAppMainLabel: "true", DockerAppLauncherLabel: "true",
	}}
	sidecar := container.Summary{ID: "side", State: container.StateRunning, Labels: map[string]string{DockerAppPathLabel: appPath}}

	for _, tc := range []struct {
		name     string
		launcher Status
		expected Status
	}{
		{"an app the launcher runs", StatusRunning, StatusRunning},
		{"a container whose launcher ran no app yet, its sidecars up", StatusStopped, StatusStopped},
		{"an app that crashed in a container that stays up", StatusFailed, StatusFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			statuses := parseAppStatus([]container.Summary{main, sidecar}, map[string]Status{"main": tc.launcher})
			require.Len(t, statuses, 1)
			assert.Equal(t, tc.expected, statuses[0].Status)
		})
	}

	t.Run("without an answer the container state stands", func(t *testing.T) {
		statuses := parseAppStatus([]container.Summary{main, sidecar}, nil)
		require.Len(t, statuses, 1)
		assert.Equal(t, StatusRunning, statuses[0].Status)
	})
}
