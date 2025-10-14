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
