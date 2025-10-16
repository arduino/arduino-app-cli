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
	"io"
	"testing"

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
