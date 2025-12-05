// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package orchestrator

import (
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"
)

func TestParseAppStatus(t *testing.T) {
	tests := []struct {
		name           string
		containerState []container.ContainerState
		want           Status
	}{
		{
			name:           "everything running",
			containerState: []container.ContainerState{container.StateRunning, container.StateRunning},
			want:           StatusRunning,
		},
		{
			name:           "everything stopped",
			containerState: []container.ContainerState{container.StateCreated, container.StatePaused, container.StateExited},
			want:           StatusStopped,
		},
		{
			name:           "failed container",
			containerState: []container.ContainerState{container.StateRunning, container.StateDead},
			want:           StatusFailed,
		},
		{
			name:           "failed container takes precedence over stopping and starting",
			containerState: []container.ContainerState{container.StateRunning, container.StateDead, container.StateRemoving, container.StateRestarting},
			want:           StatusFailed,
		},
		{
			name:           "stopping",
			containerState: []container.ContainerState{container.StateRunning, container.StateRemoving},
			want:           StatusStopping,
		},
		{
			name:           "stopping takes precedence over starting",
			containerState: []container.ContainerState{container.StateRunning, container.StateRestarting, container.StateRemoving},
			want:           StatusStopping,
		},
		{
			name:           "starting",
			containerState: []container.ContainerState{container.StateRestarting, container.StateExited},
			want:           StatusStarting,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := f.Map(tc.containerState, func(c container.ContainerState) container.Summary {
				return container.Summary{
					Labels: map[string]string{DockerAppPathLabel: "path1"},
					State:  c,
				}
			})
			res := parseAppStatus(input)
			require.Len(t, res, 1)
			require.Equal(t, tc.want, res[0].Status)
			require.Equal(t, "path1", res[0].AppPath.String())
		})
	}
}

func TestGetCustomErrorFomDockerEvent(t *testing.T) {
	tests := []struct {
		name       string
		message    string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:       "unauthorized error",
			message:    "main Error Head \"https://****/bcmi-labs/arduino/appslab-python-apps-base/manifests/0.1.16\": unauthorized",
			wantErr:    true,
			wantErrMsg: "could not reach the Docker registry to download base image. Please make sure to be authorized to download from it or flash the board with the latest Arduino Linux image. Details: main Error Head \"https://****/bcmi-labs/arduino/appslab-python-apps-base/manifests/0.1.16\": unauthorized)",
		},
		{
			name:       "connection refused error",
			message:    "main Error Get \"https://***/\": dial tcp: lookup ghcr.io on [::1]:53: read udp [::1]:52317-\u003e[::1]:53: read: connection refused",
			wantErr:    true,
			wantErrMsg: "could not reach the Docker registry to download base image. Please check your internet connection or flash the board with the latest Arduino Linux image. Details: main Error Get \"https://***/\": dial tcp: lookup ghcr.io on [::1]:53: read udp [::1]:52317-\u003e[::1]:53: read: connection refused)",
		},
		{
			name:       "no such host error",
			message:    "Get \"https://registry-1.docker.io/v2/\": dial tcp: lookup registry-1.docker.io on 127.0.0.1:53: no such host",
			wantErr:    true,
			wantErrMsg: "could not reach the Docker registry to download base image. Please check your internet connection or flash the board with the latest Arduino Linux image. Details: Get \"https://registry-1.docker.io/v2/\": dial tcp: lookup registry-1.docker.io on 127.0.0.1:53: no such host)",
		},
		{
			name:    "no matching error",
			message: "container successfully started",
			wantErr: false,
		},
		{
			name:    "empty message",
			message: "",
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := GetCustomErrorFomDockerEvent(tc.message)
			if tc.wantErr {
				require.Error(t, err)
				require.Equal(t, tc.wantErrMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
