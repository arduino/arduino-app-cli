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
		isMain         []bool
		want           Status
	}{
		{
			name:           "everything running",
			containerState: []container.ContainerState{container.StateRunning, container.StateRunning},
			statusMessage:  []string{"Up 5 minutes", "Up 10 minutes"},
			isMain:         []bool{true, false},
			want:           StatusRunning,
		},
		{
			name:           "everything stopped",
			containerState: []container.ContainerState{container.StateCreated, container.StatePaused, container.StateExited},
			statusMessage:  []string{"Created", "Paused", "Exited (137)"},
			isMain:         []bool{true, false, false},
			want:           StatusStopped,
		},
		{
			name:           "failed container",
			containerState: []container.ContainerState{container.StateRunning, container.StateDead},
			statusMessage:  []string{"Up 5 minutes", "Dead"},
			isMain:         []bool{true, false},
			want:           StatusFailed,
		},
		{
			name:           "failed container takes precedence over stopping and starting",
			containerState: []container.ContainerState{container.StateRunning, container.StateDead, container.StateRemoving, container.StateRestarting},
			statusMessage:  []string{"Up 5 minutes", "Dead", "Removing", "Restarting"},
			isMain:         []bool{true, false, false, false},
			want:           StatusFailed,
		},
		{
			name:           "stopping",
			containerState: []container.ContainerState{container.StateRunning, container.StateRemoving},
			statusMessage:  []string{"Up 5 minutes", "Removing"},
			isMain:         []bool{true, false},
			want:           StatusStopping,
		},
		{
			name:           "stopping takes precedence over starting",
			containerState: []container.ContainerState{container.StateRunning, container.StateRestarting, container.StateRemoving},
			statusMessage:  []string{"Up 5 minutes", "Restarting", "Removing"},
			isMain:         []bool{true, false, false},
			want:           StatusStopping,
		},
		{
			name:           "starting",
			containerState: []container.ContainerState{container.StateRestarting, container.StateExited},
			statusMessage:  []string{"Restarting", "Exited (129)"},
			isMain:         []bool{true, false},
			want:           StatusStarting,
		},
		{
			name:           "failed",
			containerState: []container.ContainerState{container.StateRestarting, container.StateExited},
			statusMessage:  []string{"Restarting", "Exited (1)"},
			isMain:         []bool{true, false},
			want:           StatusFailed,
		},
		{
			name:           "non-main exit 0 is considered stopped",
			containerState: []container.ContainerState{container.StateExited, container.StateExited},
			statusMessage:  []string{"Exited (137)", "Exited (0)"},
			isMain:         []bool{true, false},
			want:           StatusStopped,
		},
		{
			name:           "main exit 0 is considered failed",
			containerState: []container.ContainerState{container.StateExited, container.StateExited},
			statusMessage:  []string{"Exited (0)", "Exited (0)"},
			isMain:         []bool{true, false},
			want:           StatusFailed,
		},
		{
			name:           "non-main exit 143 (> 128) is considered stopped",
			containerState: []container.ContainerState{container.StateExited, container.StateExited},
			statusMessage:  []string{"Exited (137)", "Exited (143)"},
			isMain:         []bool{true, false},
			want:           StatusStopped,
		},
		{
			name:           "non-main exit 1 is considered failed",
			containerState: []container.ContainerState{container.StateExited, container.StateExited},
			statusMessage:  []string{"Exited (137)", "Exited (1)"},
			isMain:         []bool{true, false},
			want:           StatusFailed,
		},
		{
			name:           "unlabeled container with exit 0 is considered stopped",
			containerState: []container.ContainerState{container.StateExited},
			statusMessage:  []string{"Exited (0)"},
			isMain:         []bool{false},
			want:           StatusStopped,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var input []container.Summary
			for i, c := range tc.containerState {
				labels := map[string]string{DockerAppPathLabel: "path1"}
				if len(tc.isMain) > i && tc.isMain[i] {
					labels[DockerAppMainLabel] = "true"
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
		isMain        bool
		want          Status
	}{
		{
			name:          "running",
			state:         container.StateRunning,
			statusMessage: "Up 10 minutes",
			isMain:        true,
			want:          StatusRunning,
		},
		{
			name:          "restarting",
			state:         container.StateRestarting,
			statusMessage: "Restarting",
			isMain:        true,
			want:          StatusStarting,
		},
		{
			name:          "removing",
			state:         container.StateRemoving,
			statusMessage: "Removing",
			isMain:        true,
			want:          StatusStopping,
		},
		{
			name:          "created",
			state:         container.StateCreated,
			statusMessage: "Created",
			isMain:        true,
			want:          StatusStopped,
		},
		{
			name:          "paused",
			state:         container.StatePaused,
			statusMessage: "Paused",
			isMain:        true,
			want:          StatusStopped,
		},
		{
			name:          "dead",
			state:         container.StateDead,
			statusMessage: "Dead",
			isMain:        true,
			want:          StatusFailed,
		},
		{
			name:          "main exit 0 -> StatusFailed",
			state:         container.StateExited,
			statusMessage: "Exited (0)",
			isMain:        true,
			want:          StatusFailed,
		},
		{
			name:          "main exit > 128 -> StatusStopped",
			state:         container.StateExited,
			statusMessage: "Exited (137)",
			isMain:        true,
			want:          StatusStopped,
		},
		{
			name:          "main exit 1 -> StatusFailed",
			state:         container.StateExited,
			statusMessage: "Exited (1)",
			isMain:        true,
			want:          StatusFailed,
		},
		{
			name:          "non-main exit 0 -> StatusStopped",
			state:         container.StateExited,
			statusMessage: "Exited (0)",
			isMain:        false,
			want:          StatusStopped,
		},
		{
			name:          "non-main exit 143 (> 128) -> StatusStopped",
			state:         container.StateExited,
			statusMessage: "Exited (143)",
			isMain:        false,
			want:          StatusStopped,
		},
		{
			name:          "non-main exit 1 -> StatusFailed",
			state:         container.StateExited,
			statusMessage: "Exited (1)",
			isMain:        false,
			want:          StatusFailed,
		},
		{
			name:          "unparseable exit message -> StatusFailed",
			state:         container.StateExited,
			statusMessage: "Exited unexpectedly",
			isMain:        false,
			want:          StatusFailed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := StatusFromDockerState(tc.state, tc.statusMessage, tc.isMain)
			require.Equal(t, tc.want, got)
		})
	}
}
