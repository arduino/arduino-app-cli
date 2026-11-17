// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/stretchr/testify/assert"
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
