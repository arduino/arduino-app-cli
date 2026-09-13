// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package dockerhelper

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	dockerCommand "github.com/docker/cli/cli/command"
	"github.com/docker/compose/v5/pkg/api"
	"github.com/moby/moby/api/types/container"
	dockerClient "github.com/moby/moby/client"
	"github.com/stretchr/testify/require"
)

// TestComposeRoundTrip starts a project on the engine and stops it the way an app is
// started and stopped: through the sdk, and by project name alone.
func TestComposeRoundTrip(t *testing.T) {
	docker := getDockerCli(t)
	const projectName = "arduino-app-cli-round-trip"

	prj := &types.Project{
		Name:         projectName,
		WorkingDir:   t.TempDir(),
		ComposeFiles: []string{"app-compose.yaml"},
		Services: types.Services{
			"main": types.ServiceConfig{
				Name:    "main",
				Image:   "busybox:latest",
				Command: types.ShellCommand{"sleep", "600"},
				// What an app is found by, and what its services are read from
				Labels: types.Labels{"cc.arduino.app": "true", "cc.arduino.app.path": "/tmp/round-trip"},
			},
		},
	}
	t.Cleanup(func() {
		_ = ComposeDown(t.Context(), docker, projectName, nil)
	})

	require.NoError(t, ComposeUp(t.Context(), docker, prj, nil))

	// The labels are what compose finds its containers by, and what an app is read from.
	containers, err := docker.Client().ContainerList(t.Context(), dockerClient.ContainerListOptions{
		All:     true,
		Filters: make(dockerClient.Filters).Add("label", "com.docker.compose.project="+projectName),
	})
	require.NoError(t, err)
	require.Len(t, containers.Items, 1)
	require.Equal(t, container.StateRunning, containers.Items[0].State)
	require.Equal(t, "main", containers.Items[0].Labels["com.docker.compose.service"])

	require.NoError(t, ComposeStop(t.Context(), docker, projectName, nil))
	stopped, err := docker.Client().ContainerInspect(t.Context(), containers.Items[0].ID, dockerClient.ContainerInspectOptions{})
	require.NoError(t, err)
	require.Equal(t, container.StateExited, stopped.Container.State.Status)

	require.NoError(t, ComposeDown(t.Context(), docker, projectName, nil))
	gone, err := docker.Client().ContainerList(t.Context(), dockerClient.ContainerListOptions{
		All:     true,
		Filters: make(dockerClient.Filters).Add("label", "com.docker.compose.project="+projectName),
	})
	require.NoError(t, err)
	require.Empty(t, gone.Items)
}

func TestComposeProgressSaysAThingOnce(t *testing.T) {
	var said []string
	progress := newProgress(func(line string) { said = append(said, line) })

	progress.On(
		api.Resource{ID: "layer", Text: "Downloading", Current: 1, Total: 100},
		api.Resource{ID: "layer", Text: "Downloading", Current: 50, Total: 100},
		api.Resource{ID: "layer", Text: "Downloading", Current: 99, Total: 100},
		api.Resource{ID: "layer", Text: "Pull complete"},
		api.Resource{ID: "main", Text: "Created"},
	)

	require.Equal(t, []string{"layer Downloading", "layer Pull complete", "main Created"}, said)
}

func TestComposeProgressKeepsTheRegistryError(t *testing.T) {
	progress := newProgress(nil)

	progress.On(api.Resource{ID: "main", Text: "Error", Details: "Head \"https://ghcr.io/v2/\": unauthorized"})

	require.ErrorContains(t, progress.registryError, "make sure to be authorized")
}

func TestRegistryError(t *testing.T) {
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
			err := registryError(tc.message)
			if tc.wantErr {
				require.Error(t, err)
				require.Equal(t, tc.wantErrMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func getDockerCli(t *testing.T) dockerCommand.Cli {
	t.Helper()
	docker, err := NewCli()
	require.NoError(t, err)
	return docker
}
