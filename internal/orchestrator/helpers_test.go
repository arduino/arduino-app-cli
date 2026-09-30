// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/stretchr/testify/require"
)

func TestParseAppStatus(t *testing.T) {
	tests := []struct {
		name           string
		containerState []container.ContainerState
		statusMessage  []string
		serviceNames   []string
		want           Status
	}{
		{
			name:           "everything running",
			containerState: []container.ContainerState{container.StateRunning, container.StateRunning},
			statusMessage:  []string{"Up 5 minutes", "Up 10 minutes"},
			serviceNames:   []string{"main", "dep"},
			want:           StatusRunning,
		},
		{
			name:           "everything stopped",
			containerState: []container.ContainerState{container.StateCreated, container.StatePaused, container.StateExited},
			statusMessage:  []string{"Created", "Paused", "Exited (137)"},
			serviceNames:   []string{"main", "dep1", "dep2"},
			want:           StatusStopped,
		},
		{
			name:           "failed container",
			containerState: []container.ContainerState{container.StateRunning, container.StateDead},
			statusMessage:  []string{"Up 5 minutes", "Dead"},
			serviceNames:   []string{"main", "dep"},
			want:           StatusFailed,
		},
		{
			name:           "failed container takes precedence over stopping and starting",
			containerState: []container.ContainerState{container.StateRunning, container.StateDead, container.StateRemoving, container.StateRestarting},
			statusMessage:  []string{"Up 5 minutes", "Dead", "Removing", "Restarting"},
			serviceNames:   []string{"main", "dep1", "dep2", "dep3"},
			want:           StatusFailed,
		},
		{
			name:           "stopping",
			containerState: []container.ContainerState{container.StateRunning, container.StateRemoving},
			statusMessage:  []string{"Up 5 minutes", "Removing"},
			serviceNames:   []string{"main", "dep"},
			want:           StatusStopping,
		},
		{
			name:           "stopping takes precedence over starting",
			containerState: []container.ContainerState{container.StateRunning, container.StateRestarting, container.StateRemoving},
			statusMessage:  []string{"Up 5 minutes", "Restarting", "Removing"},
			serviceNames:   []string{"main", "dep1", "dep2"},
			want:           StatusStopping,
		},
		{
			name:           "starting",
			containerState: []container.ContainerState{container.StateRestarting, container.StateExited},
			statusMessage:  []string{"Restarting", "Exited (129)"},
			serviceNames:   []string{"main", "dep"},
			want:           StatusStarting,
		},
		{
			name:           "failed",
			containerState: []container.ContainerState{container.StateRestarting, container.StateExited},
			statusMessage:  []string{"Restarting", "Exited (1)"},
			serviceNames:   []string{"main", "dep"},
			want:           StatusFailed,
		},
		{
			name:           "non-main exit 0 is considered stopped",
			containerState: []container.ContainerState{container.StateExited, container.StateExited},
			statusMessage:  []string{"Exited (137)", "Exited (0)"},
			serviceNames:   []string{"main", "init"},
			want:           StatusStopped,
		},
		{
			name:           "main exit 0 is considered failed",
			containerState: []container.ContainerState{container.StateExited, container.StateExited},
			statusMessage:  []string{"Exited (0)", "Exited (0)"},
			serviceNames:   []string{"main", "init"},
			want:           StatusFailed,
		},
		{
			name:           "non-main exit 143 (> 128) is considered stopped",
			containerState: []container.ContainerState{container.StateExited, container.StateExited},
			statusMessage:  []string{"Exited (137)", "Exited (143)"},
			serviceNames:   []string{"main", "dep"},
			want:           StatusStopped,
		},
		{
			name:           "non-main exit 1 is considered failed",
			containerState: []container.ContainerState{container.StateExited, container.StateExited},
			statusMessage:  []string{"Exited (137)", "Exited (1)"},
			serviceNames:   []string{"main", "dep"},
			want:           StatusFailed,
		},
		{
			name:           "empty serviceName with exit 0 is considered failed",
			containerState: []container.ContainerState{container.StateExited},
			statusMessage:  []string{"Exited (0)"},
			serviceNames:   []string{""},
			want:           StatusFailed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var input []container.Summary
			for i, c := range tc.containerState {
				labels := map[string]string{DockerAppPathLabel: "path1"}
				if len(tc.serviceNames) > i && tc.serviceNames[i] != "" {
					labels[dockerComposeServiceLabel] = tc.serviceNames[i]
				}
				input = append(input, container.Summary{
					Labels: labels,
					State:  c,
					Status: tc.statusMessage[i],
				})
			}
			res := parseAppStatus(input)
			require.Len(t, res, 1)
			require.Equal(t, tc.want, res[0].Status)
			require.Equal(t, "path1", res[0].AppPath.String())
		})
	}
}

func TestStatusFromDockerState(t *testing.T) {
	tests := []struct {
		name          string
		state         container.ContainerState
		statusMessage string
		serviceName   string
		want          Status
	}{
		{
			name:          "running",
			state:         container.StateRunning,
			statusMessage: "Up 10 minutes",
			serviceName:   "main",
			want:          StatusRunning,
		},
		{
			name:          "restarting",
			state:         container.StateRestarting,
			statusMessage: "Restarting",
			serviceName:   "main",
			want:          StatusStarting,
		},
		{
			name:          "removing",
			state:         container.StateRemoving,
			statusMessage: "Removing",
			serviceName:   "main",
			want:          StatusStopping,
		},
		{
			name:          "created",
			state:         container.StateCreated,
			statusMessage: "Created",
			serviceName:   "main",
			want:          StatusStopped,
		},
		{
			name:          "paused",
			state:         container.StatePaused,
			statusMessage: "Paused",
			serviceName:   "main",
			want:          StatusStopped,
		},
		{
			name:          "dead",
			state:         container.StateDead,
			statusMessage: "Dead",
			serviceName:   "main",
			want:          StatusFailed,
		},
		{
			name:          "main exit 0 -> StatusFailed",
			state:         container.StateExited,
			statusMessage: "Exited (0)",
			serviceName:   "main",
			want:          StatusFailed,
		},
		{
			name:          "main exit > 128 -> StatusStopped",
			state:         container.StateExited,
			statusMessage: "Exited (137)",
			serviceName:   "main",
			want:          StatusStopped,
		},
		{
			name:          "main exit 1 -> StatusFailed",
			state:         container.StateExited,
			statusMessage: "Exited (1)",
			serviceName:   "main",
			want:          StatusFailed,
		},
		{
			name:          "non-main exit 0 -> StatusStopped",
			state:         container.StateExited,
			statusMessage: "Exited (0)",
			serviceName:   "init",
			want:          StatusStopped,
		},
		{
			name:          "non-main exit 143 (> 128) -> StatusStopped",
			state:         container.StateExited,
			statusMessage: "Exited (143)",
			serviceName:   "worker",
			want:          StatusStopped,
		},
		{
			name:          "non-main exit 1 -> StatusFailed",
			state:         container.StateExited,
			statusMessage: "Exited (1)",
			serviceName:   "worker",
			want:          StatusFailed,
		},
		{
			name:          "empty serviceName with exit 0 -> StatusFailed",
			state:         container.StateExited,
			statusMessage: "Exited (0)",
			serviceName:   "",
			want:          StatusFailed,
		},
		{
			name:          "empty serviceName with exit 143 (> 128) -> StatusStopped",
			state:         container.StateExited,
			statusMessage: "Exited (143)",
			serviceName:   "",
			want:          StatusStopped,
		},
		{
			name:          "empty serviceName with exit 1 -> StatusFailed",
			state:         container.StateExited,
			statusMessage: "Exited (1)",
			serviceName:   "",
			want:          StatusFailed,
		},
		{
			name:          "unparseable exit message -> StatusFailed",
			state:         container.StateExited,
			statusMessage: "Exited unexpectedly",
			serviceName:   "worker",
			want:          StatusFailed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := StatusFromDockerState(tc.state, tc.statusMessage, tc.serviceName)
			require.Equal(t, tc.want, got)
		})
	}
}
