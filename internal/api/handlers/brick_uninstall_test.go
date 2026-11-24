// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/moby/moby/api/types/container"
	dockerClient "github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

// fakeContainerClient is a docker client that only lists containers: any other call panics.
type fakeContainerClient struct {
	dockerClient.APIClient

	containers []container.Summary
	err        error
}

func (f *fakeContainerClient) ContainerList(context.Context, dockerClient.ContainerListOptions) (dockerClient.ContainerListResult, error) {
	return dockerClient.ContainerListResult{Items: f.containers}, f.err
}

// Ping reports the daemon as unreachable, so that the initialization of the cli does not
// negotiate the API version with the fake client.
func (f *fakeContainerClient) Ping(context.Context, dockerClient.PingOptions) (dockerClient.PingResult, error) {
	return dockerClient.PingResult{}, errors.New("docker daemon not reachable")
}

func newFakeDockerCli(t *testing.T, client *fakeContainerClient) command.Cli {
	t.Helper()
	cli, err := command.NewDockerCli(command.WithAPIClient(client), command.WithBaseContext(t.Context()))
	require.NoError(t, err)
	require.NoError(t, cli.Initialize(&flags.ClientOptions{}))
	return cli
}

// uninstallBrick calls the route with docker, if not nil. The installed bricks have no compose
// file, so the uninstall removes no image and the fake client is enough.
func uninstallBrick(t *testing.T, cfg config.Configuration, docker *fakeContainerClient, target string) *httptest.ResponseRecorder {
	t.Helper()
	var cli command.Cli
	if docker != nil {
		cli = newFakeDockerCli(t, docker)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, target, nil)
	req.SetPathValue("brickID", strings.TrimPrefix(strings.SplitN(target, "?", 2)[0], "/v1/bricks/"))
	bricksIndex := (&bricksindex.BricksIndex{}).WithInstalledBricks(cfg.BricksDir())
	HandleUninstallBrick(cli, cfg, bricksIndex, nil, nil)(rec, req)
	return rec
}

func TestHandleUninstallBrick(t *testing.T) {
	installed := func(t *testing.T, versions ...string) config.Configuration {
		cfg := brickTestConfig(t, nil, "")
		for _, v := range versions {
			require.NoError(t, cfg.BricksDir().Join("acme", "camera", v).MkdirAll())
		}
		return cfg
	}

	// Without docker: they are answered before the uninstall uses it.
	tests := []struct {
		name       string
		target     string
		wantStatus int
		wantBody   string
	}{
		{"invalid id", "/v1/bricks/acme:Camera", http.StatusBadRequest, "invalid brick id"},
		{"id without namespace", "/v1/bricks/camera", http.StatusBadRequest, "invalid brick id"},
		{"built-in brick", "/v1/bricks/arduino:camera", http.StatusBadRequest, "the brick arduino:camera is built-in and cannot be uninstalled"},
		{"empty version", "/v1/bricks/acme:camera?version=", http.StatusBadRequest, "missing version"},
		{"invalid version", "/v1/bricks/acme:camera?version=v1", http.StatusBadRequest, "invalid version"},
		{"brick not installed", "/v1/bricks/acme:audio", http.StatusNotFound, "the brick acme:audio is not installed"},
		{"version not installed", "/v1/bricks/acme:camera?version=2.0.0", http.StatusNotFound, "the brick acme:camera@2.0.0 is not installed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := uninstallBrick(t, installed(t, "1.0.0"), nil, tt.target)
			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Contains(t, rec.Body.String(), tt.wantBody)
		})
	}

	t.Run("every version", func(t *testing.T) {
		cfg := installed(t, "1.0.0", "1.1.0")
		rec := uninstallBrick(t, cfg, &fakeContainerClient{}, "/v1/bricks/acme:camera")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"id":"acme:camera","versions":["1.0.0","1.1.0"],"warnings":[]}`, rec.Body.String())
		assert.True(t, cfg.BricksDir().Join("acme").NotExist())
	})

	t.Run("one version, compared as semver", func(t *testing.T) {
		cfg := installed(t, "1.0.0", "1.1.0")
		rec := uninstallBrick(t, cfg, &fakeContainerClient{}, "/v1/bricks/acme:camera?version=1.1")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"id":"acme:camera","versions":["1.1.0"],"warnings":[]}`, rec.Body.String())
		assert.True(t, cfg.BricksDir().Join("acme", "camera", "1.0.0").Exist())
	})

	t.Run("used by a running app", func(t *testing.T) {
		cfg := installed(t, "1.0.0")
		docker := &fakeContainerClient{containers: []container.Summary{{
			Labels: map[string]string{orchestrator.DockerAppPathLabel: "/apps/demo"},
			State:  container.StateRunning,
			Mounts: []container.MountPoint{{Source: cfg.BricksDir().Join("acme", "camera", "1.0.0", "src", "camera").String()}},
		}}}
		rec := uninstallBrick(t, cfg, docker, "/v1/bricks/acme:camera")
		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Contains(t, rec.Body.String(), "acme:camera@1.0.0 is used by the app at /apps/demo")
		assert.True(t, cfg.BricksDir().Join("acme", "camera", "1.0.0").Exist())
	})

	t.Run("docker failure", func(t *testing.T) {
		cfg := installed(t, "1.0.0")
		rec := uninstallBrick(t, cfg, &fakeContainerClient{err: errors.New("docker unreachable")}, "/v1/bricks/acme:camera")
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Contains(t, rec.Body.String(), "docker unreachable")
		assert.True(t, cfg.BricksDir().Join("acme", "camera", "1.0.0").Exist())
	})
}
