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
		want           Status
	}{
		{
			name:           "everything running",
			containerState: []container.ContainerState{container.StateRunning, container.StateRunning},
			statusMessage:  []string{"Up 5 minutes", "Up 10 minutes"},
			want:           StatusRunning,
		},
		{
			name:           "everything stopped",
			containerState: []container.ContainerState{container.StateCreated, container.StatePaused, container.StateExited},
			statusMessage:  []string{"Created", "Paused", "Exited (137)"},
			want:           StatusStopped,
		},
		{
			name:           "failed container",
			containerState: []container.ContainerState{container.StateRunning, container.StateDead},
			statusMessage:  []string{"Up 5 minutes", "Dead"},
			want:           StatusFailed,
		},
		{
			name:           "failed container takes precedence over stopping and starting",
			containerState: []container.ContainerState{container.StateRunning, container.StateDead, container.StateRemoving, container.StateRestarting},
			statusMessage:  []string{"Up 5 minutes", "Dead", "Removing", "Restarting"},
			want:           StatusFailed,
		},
		{
			name:           "stopping",
			containerState: []container.ContainerState{container.StateRunning, container.StateRemoving},
			statusMessage:  []string{"Up 5 minutes", "Removing"},
			want:           StatusStopping,
		},
		{
			name:           "stopping takes precedence over starting",
			containerState: []container.ContainerState{container.StateRunning, container.StateRestarting, container.StateRemoving},
			statusMessage:  []string{"Up 5 minutes", "Restarting", "Removing"},
			want:           StatusStopping,
		},
		{
			name:           "starting",
			containerState: []container.ContainerState{container.StateRestarting, container.StateExited},
			statusMessage:  []string{"Restarting", "Exited (129)"},
			want:           StatusStarting,
		},
		{
			name:           "failed",
			containerState: []container.ContainerState{container.StateRestarting, container.StateExited},
			statusMessage:  []string{"Restarting", "Exited (1)"},
			want:           StatusFailed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var input []container.Summary
			for i, c := range tc.containerState {
				input = append(input, container.Summary{
					Labels: map[string]string{DockerAppPathLabel: "path1"},
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

func TestParseAppStatusServiceExit(t *testing.T) {
	const path = "path1"
	for _, tt := range []struct {
		name, service, status string
		want                  Status
	}{
		{"clean sidecar shutdown", "llamacpp-models-runner", "Exited (0) 5 seconds ago", StatusStopped},
		{"sidecar failure", "llamacpp-models-runner", "Exited (1) 5 seconds ago", StatusFailed},
		{"main returned normally", "main", "Exited (0) 5 seconds ago", StatusFailed},
		{"unknown service returned normally", "", "Exited (0) 5 seconds ago", StatusFailed},
		{"sidecar interrupted by signal", "llamacpp-models-runner", "Exited (143) 5 seconds ago", StatusStopped},
		{"unrecognized sidecar exit", "llamacpp-models-runner", "Exited", StatusFailed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			containers := []container.Summary{
				{
					Labels: map[string]string{DockerAppPathLabel: path, "com.docker.compose.service": "main"},
					State:  container.StateExited, Status: "Exited (143) 5 seconds ago",
				},
				{
					Labels: map[string]string{DockerAppPathLabel: path, "com.docker.compose.service": tt.service},
					State:  container.StateExited, Status: tt.status,
				},
			}
			res := parseAppStatus(containers)
			require.Len(t, res, 1)
			require.Equal(t, tt.want, res[0].Status)
		})
	}
}
