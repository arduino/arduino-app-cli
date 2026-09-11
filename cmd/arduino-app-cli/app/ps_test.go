// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

func TestFilterAppsByStatus(t *testing.T) {
	appWithStatus := func(name string, status orchestrator.Status) orchestrator.AppInfo {
		return orchestrator.AppInfo{Name: name, Status: status}
	}

	apps := []orchestrator.AppInfo{
		appWithStatus("starting-app", orchestrator.StatusStarting),
		appWithStatus("running-app", orchestrator.StatusRunning),
		appWithStatus("stopping-app", orchestrator.StatusStopping),
		appWithStatus("failed-app", orchestrator.StatusFailed),
		appWithStatus("stopped-app", orchestrator.StatusStopped),
		appWithStatus("never-started-app", orchestrator.StatusUninitialized),
	}

	t.Run("default excludes stopped and uninitialized apps", func(t *testing.T) {
		got := filterAppsByStatus(apps, false)
		names := namesOf(got)
		assert.ElementsMatch(t, []string{"starting-app", "running-app", "stopping-app", "failed-app"}, names)
	})

	t.Run("--all includes stopped but never uninitialized apps", func(t *testing.T) {
		got := filterAppsByStatus(apps, true)
		names := namesOf(got)
		assert.ElementsMatch(t, []string{
			"starting-app", "running-app", "stopping-app", "failed-app", "stopped-app",
		}, names)
	})
}

func namesOf(apps []orchestrator.AppInfo) []string {
	names := make([]string, 0, len(apps))
	for _, a := range apps {
		names = append(names, a.Name)
	}
	return names
}
