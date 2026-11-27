// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"errors"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/moby/moby/api/types/container"
	"github.com/stretchr/testify/require"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
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
	require.ErrorIs(t, check(running, "acme:camera", nil), ErrBrickInUse)
	require.ErrorContains(t, check(running, "acme:camera", nil), "acme:camera@1.0.0 is used by the app at /apps/demo")
	require.ErrorContains(t, check(running, "acme:camera", semver.MustParse("1.0.0")), "acme:camera@1.0.0 is used")
	require.ErrorContains(t, check(running, "acme:camera", semver.MustParse("1.0")), "acme:camera@1.0.0 is used", "the version is compared as semver")
	require.NoError(t, check(running, "acme:camera", semver.MustParse("1.1.0")))
	require.NoError(t, check(running, "acme:cam", nil), "a brick whose name is a prefix of another")
	require.NoError(t, check(running, "other:camera", nil))

	stopped := []container.Summary{appContainer(container.StateExited, camera), appContainer(container.StateCreated, camera)}
	require.NoError(t, check(stopped, "acme:camera", nil))

	// A docker failure is not reported as the brick being in use.
	err := checkBrickNotInUse(t.Context(), &fakeContainerClient{err: errors.New("docker unreachable")}, bricksDir, "acme:camera", nil)
	require.ErrorContains(t, err, "docker unreachable")
	require.NotErrorIs(t, err, ErrBrickInUse)
}

func TestInstalledBrickVersionsImages(t *testing.T) {
	bricksDir := paths.New(t.TempDir())
	for version, image := range map[string]string{"1.0.0": "ghcr.io/acme/camera:1.0.0", "1.2.0": "ghcr.io/acme/camera:1.2.0"} {
		srcDir := bricksDir.Join("acme", "camera", version, "src", "camera")
		require.NoError(t, srcDir.MkdirAll())
		require.NoError(t, srcDir.Join("brick_config.yaml").WriteFile([]byte("id: acme:camera\n")))
		require.NoError(t, srcDir.Join("brick_compose.yaml").WriteFile([]byte("services:\n  camera:\n    image: "+image+"\n")))
	}
	bricksIndex := (&bricksindex.BricksIndex{}).WithInstalledBricks(bricksDir)
	images := func(version *semver.Version) map[string][]string {
		versions, err := bricksindex.ListInstalledVersions(bricksDir, "acme:camera", version)
		require.NoError(t, err)
		return installedBrickVersionsImages(bricksIndex, "acme:camera", versions, func(msg string) {
			t.Errorf("unexpected warning: %s", msg)
		})
	}

	// The version is compared as semver: 1.2 is the version installed in the folder 1.2.0.
	require.Equal(t, map[string][]string{"1.2.0": {"ghcr.io/acme/camera:1.2.0"}}, images(semver.MustParse("1.2")))
	require.Equal(t, map[string][]string{
		"1.0.0": {"ghcr.io/acme/camera:1.0.0"},
		"1.2.0": {"ghcr.io/acme/camera:1.2.0"},
	}, images(nil))
}

func TestUninstallBrickNotInstalled(t *testing.T) {
	tmp := paths.New(t.TempDir())
	t.Setenv("ARDUINO_APP_CLI__DATA_DIR", tmp.Join("data").String())
	t.Setenv("ARDUINO_APP_CLI__APPS_DIR", tmp.Join("apps").String())
	cfg, err := config.NewFromEnv()
	require.NoError(t, err)
	require.NoError(t, cfg.BricksDir().Join("acme", "camera", "1.0.0").MkdirAll())

	// The docker client is nil, so the uninstall fails if it uses it: a brick that is not
	// installed is reported without docker, even when docker is not available.
	uninstall := func(id string, version *semver.Version) (bricksindex.UninstallResult, error) {
		return UninstallBrick(t.Context(), nil, cfg, nil, nil, nil, id, version, func(msg string) {
			t.Errorf("unexpected warning: %s", msg)
		})
	}
	res, err := uninstall("acme:audio", nil)
	require.NoError(t, err)
	require.Empty(t, res.Versions)

	res, err = uninstall("acme:camera", semver.MustParse("2.0.0"))
	require.NoError(t, err)
	require.Empty(t, res.Versions)

	_, err = uninstall("camera", nil)
	require.ErrorContains(t, err, "invalid brick id")
}
