// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package orchestrator

import (
	"io"
	"testing"

	"github.com/arduino/go-paths-helper"
	dockerCommand "github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/docker/api/types/image"
	dockerClient "github.com/docker/docker/client"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"
)

func TestListImagesAlreadyPulled(t *testing.T) {
	docker := getDockerClient(t)

	r, err := docker.ImagePull(t.Context(), "ghcr.io/arduino/app-bricks/python-apps-base:0.4.8", image.PullOptions{})
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, r)
	r.Close()

	images, err := listImagesAlreadyPulled(t.Context(), docker)
	require.NoError(t, err)
	require.Contains(t, images, "ghcr.io/arduino/app-bricks/python-apps-base:0.4.8")
}

func TestRemoveImage(t *testing.T) {
	docker := getDockerClient(t)

	r, err := docker.ImagePull(t.Context(), "ghcr.io/arduino/app-bricks/python-apps-base:0.4.8", image.PullOptions{})
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, r)
	r.Close()

	size, err := removeImage(t.Context(), docker, "ghcr.io/arduino/app-bricks/python-apps-base:0.4.8")
	require.NoError(t, err)
	require.Greater(t, size, int64(1024))
}

func getDockerClient(t *testing.T) dockerClient.APIClient {
	t.Helper()
	d, err := dockerCommand.NewDockerCli(
		dockerCommand.WithAPIClient(
			f.Must(dockerClient.NewClientWithOpts(
				dockerClient.FromEnv,
				dockerClient.WithAPIVersionNegotiation(),
			)),
		),
	)
	require.NoError(t, err)
	err = d.Initialize(flags.NewClientOptions())
	require.NoError(t, err)
	return d.Client()
}

func TestExtractImagesFromCompose(t *testing.T) {
	oldPrefixes := imagePrefixes
	imagePrefixes = []string{"ghcr.io/bcmi-labs/", "public.ecr.aws/arduino/", "ghcr.io/arduino/", "influxdb"}
	defer func() { imagePrefixes = oldPrefixes }()

	tests := []struct {
		name           string
		composePath    *paths.Path
		expectedImages []string
		wantErr        bool
	}{
		{
			name:           "valid compose with supported images",
			composePath:    paths.New("testdata", "composes", "service_compose_valid.yaml"),
			expectedImages: []string{"ghcr.io/arduino/app-bricks/ollama-models-runner:dev-next"},
			wantErr:        false,
		},
		{
			name:        "invalid compose",
			composePath: paths.New("testdata", "composes", "service_compose_invalid.yaml"),
			wantErr:     true,
		},
		{
			name:           "no matching prefixes",
			composePath:    paths.New("testdata", "composes", "service_compose_no_prefix_match.yaml"),
			expectedImages: nil,
			wantErr:        false,
		},
		{
			name:        "multiple services with mixed prefixes",
			composePath: paths.New("testdata", "composes", "service_compose_multiple.yaml"),
			expectedImages: []string{
				"ghcr.io/arduino/app-bricks/genie-models-runner:dev-next",
				"ghcr.io/arduino/app-bricks/audio-analytics-models-runner:dev-next",
			},
			wantErr: false,
		},
		{
			name:        "file not found",
			composePath: paths.New("testdata", "composes", "does_not_exist.yaml"),
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := extractImagesFromCompose(tt.composePath)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, tt.expectedImages, got)
			}
		})
	}
}
