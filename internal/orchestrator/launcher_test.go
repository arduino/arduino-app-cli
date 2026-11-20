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

func TestTheRuntimeFolderOfAnAppIsNamedWithinTheRoomItIsGiven(t *testing.T) {
	assert.Equal(t, "my-app", runtimeKey("my-app", 60))

	long := strings.Repeat("a-very-long-app-name-", 5)
	key := runtimeKey(long, 60)
	assert.LessOrEqual(t, len(key), 60)
	assert.True(t, strings.HasPrefix(key, "a-very-long-app-name-"))
	assert.NotEqual(t, key, runtimeKey(long+"x", 60), "two long names are told apart")

	assert.Len(t, runtimeKey(long, 5), 12, "no room for a name leaves the hash alone")
}

func TestTheRuntimeFolderOfAnAppIsInTheDataDir(t *testing.T) {
	cfg := setTestOrchestratorConfig(t)
	dir := appRuntimeDir(cfg, cfg.AppsDir().Join("my-app"))
	assert.Equal(t, cfg.DataDir().Join("run").String(), dir.Parent().String())
}

func TestLauncherServedReadsTheMainServiceLabel(t *testing.T) {
	served := &types.Project{Services: types.Services{"main": {Labels: types.Labels{DockerAppLauncherLabel: "true"}}}}
	assert.True(t, launcherServed(served))
	classic := &types.Project{Services: types.Services{"main": {Labels: types.Labels{DockerAppMainLabel: "true"}}}}
	assert.False(t, launcherServed(classic), "a release frozen before the launcher runs its app the classic way")
	assert.False(t, launcherServed(&types.Project{}))
}
