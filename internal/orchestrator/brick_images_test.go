// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/moby/moby/api/types/container"
	"github.com/stretchr/testify/require"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
)

func TestBrickImages(t *testing.T) {
	const image = "ghcr.io/acme/camera:1.0@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	composeFile := paths.New(t.TempDir(), "brick_compose.yaml")
	require.NoError(t, composeFile.WriteFile([]byte("services:\n  camera:\n    image: "+image+"\n")))
	services := &servicesindex.ServicesIndex{
		Services: []servicesindex.Service{
			{ServiceID: "arduino:genie", ComposeFile: paths.New("testdata", "composes", "service_compose_genie.yaml")},
		},
	}

	// The images that are not ours are included: a third-party brick runs its own.
	images, err := BrickImages(bricksindex.Brick{
		ID:               "acme:camera",
		ComposeFile:      composeFile,
		RequiresServices: bricksindex.RequiresServices{{ID: "arduino:genie"}, {ID: "arduino:missing"}},
	}, services)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{image, "ghcr.io/arduino/app-bricks/genie-service:1.0.0"}, images)

	images, err = BrickImages(bricksindex.Brick{ID: "acme:empty"}, services)
	require.NoError(t, err)
	require.Empty(t, images)
}

func TestCheckBrickNotInUse(t *testing.T) {
	bricksDir := paths.New(t.TempDir())
	appContainer := func(state container.ContainerState, source *paths.Path) container.Summary {
		return container.Summary{
			Labels: map[string]string{DockerAppPathLabel: "/apps/demo"},
			State:  state,
			Mounts: []container.MountPoint{{Source: "/dev"}, {Source: source.String()}},
		}
	}
	camera := bricksDir.Join("acme", "camera", "1.0.0", "src", "camera")
	check := func(containers []container.Summary, id string, version *semver.Version) error {
		return checkBrickNotInUse(t.Context(), &fakeContainerClient{containers: containers}, bricksDir, id, version)
	}

	running := []container.Summary{appContainer(container.StateRunning, camera)}
	require.ErrorContains(t, check(running, "acme:camera", nil), "acme:camera@1.0.0 is used by the app at /apps/demo")
	require.ErrorContains(t, check(running, "acme:camera", semver.MustParse("1.0.0")), "acme:camera@1.0.0 is used")
	require.NoError(t, check(running, "acme:camera", semver.MustParse("1.1.0")))
	require.NoError(t, check(running, "acme:cam", nil), "a brick whose name is a prefix of another")
	require.NoError(t, check(running, "other:camera", nil))

	stopped := []container.Summary{appContainer(container.StateExited, camera), appContainer(container.StateCreated, camera)}
	require.NoError(t, check(stopped, "acme:camera", nil))
}
