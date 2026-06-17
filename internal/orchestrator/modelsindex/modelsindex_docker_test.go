// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package modelsindex

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/platform"
)

// fakeDockerClient intercepts ContainerCreate/Start/Logs/Inspect/Remove.
// All other client.APIClient methods panic — they must not be called.
type fakeDockerClient struct {
	client.APIClient

	runFunc func(image string, cmd []string) (stdout string, exitCode int)

	mu        sync.Mutex
	idCounter int
	pending   map[string]pendingContainer
	results   map[string]containerRun
}

type pendingContainer struct {
	image string
	cmd   []string
}

type containerRun struct {
	stdout   string
	exitCode int
}

func newFakeDockerClient(runFunc func(image string, cmd []string) (stdout string, exitCode int)) *fakeDockerClient {
	return &fakeDockerClient{
		runFunc: runFunc,
		pending: make(map[string]pendingContainer),
		results: make(map[string]containerRun),
	}
}

func (f *fakeDockerClient) ContainerCreate(_ context.Context, cfg *container.Config, _ *container.HostConfig, _ *network.NetworkingConfig, _ *specs.Platform, _ string) (container.CreateResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.idCounter++
	id := fmt.Sprintf("fake-%d", f.idCounter)
	f.pending[id] = pendingContainer{image: cfg.Image, cmd: cfg.Cmd}
	return container.CreateResponse{ID: id}, nil
}

func (f *fakeDockerClient) ContainerStart(_ context.Context, id string, _ container.StartOptions) error {
	f.mu.Lock()
	p := f.pending[id]
	delete(f.pending, id)
	f.mu.Unlock()

	stdout, code := f.runFunc(p.image, p.cmd)

	f.mu.Lock()
	f.results[id] = containerRun{stdout: stdout, exitCode: code}
	f.mu.Unlock()
	return nil
}

func (f *fakeDockerClient) ContainerLogs(_ context.Context, id string, _ container.LogsOptions) (io.ReadCloser, error) {
	f.mu.Lock()
	r := f.results[id]
	f.mu.Unlock()

	var buf bytes.Buffer
	w := stdcopy.NewStdWriter(&buf, stdcopy.Stdout)
	if r.stdout != "" {
		fmt.Fprint(w, r.stdout)
	}
	return io.NopCloser(&buf), nil
}

func (f *fakeDockerClient) ContainerInspect(_ context.Context, id string) (container.InspectResponse, error) {
	f.mu.Lock()
	r := f.results[id]
	f.mu.Unlock()

	return container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			State: &container.State{ExitCode: r.exitCode},
		},
	}, nil
}

func (f *fakeDockerClient) ContainerRemove(_ context.Context, id string, _ container.RemoveOptions) error {
	f.mu.Lock()
	delete(f.results, id)
	f.mu.Unlock()
	return nil
}

// loadHandlersTestIndex loads a ModelsIndex from testdata/with-handlers using the given fake client.
func loadHandlersTestIndex(t *testing.T, dockerCli client.APIClient) *ModelsIndex {
	t.Helper()
	dir := paths.New("testdata/with-handlers")
	modelsDir := dir.Join("models")
	idx, err := Load(platform.Platform{BoardName: "ventunoq"}, dir, modelsDir, dockerCli, "")
	require.NoError(t, err)
	return idx
}

func TestGetModelByID_WithDockerMock(t *testing.T) {
	t.Run("the custom modeldir volume is not resolved at load time", func(t *testing.T) {
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
			return "", 0
		})
		idx := loadHandlersTestIndex(t, cli)

		require.Equal(t, []string{"${ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR}:/models"}, idx.Handlers.listing.Volumes)
		h, ok := idx.Handlers.GetHandlerByID("ai-hub-handler")
		require.True(t, ok)
		require.Equal(t, []string{"${ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR}:/models"}, h.Volumes)

	})

	t.Run("piper-tts-en is pre-loaded: returns size from metadata, no Docker call", func(t *testing.T) {
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
			t.Fatal("unexpected Docker call for pre-loaded model")
			return "", 0
		})
		idx := loadHandlersTestIndex(t, cli)

		model, found, err := idx.GetModelByID(context.Background(), "piper-tts-en")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, uint64(46*1024*1024), model.Size)
	})

	t.Run("ei:efficientnet-b4 not installed: check exits 1 with error event", func(t *testing.T) {
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
			// check action → model not present (script signals this via error event + exit 1)
			return "{\"event\":\"error\",\"description\":\"model not installed\"}\n", 1
		})
		idx := loadHandlersTestIndex(t, cli)

		model, found, err := idx.GetModelByID(context.Background(), "ei:efficientnet-b4")
		require.NoError(t, err)
		require.True(t, found)
		assert.False(t, model.Installed)
		assert.Equal(t, uint64(89*1024*1024), model.Size)
	})

	t.Run("ei:efficientnet-b4 installed: check exits 0, size from metadata", func(t *testing.T) {
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
			// check action → model is present
			return "", 0
		})
		idx := loadHandlersTestIndex(t, cli)

		model, found, err := idx.GetModelByID(context.Background(), "ei:efficientnet-b4")
		require.NoError(t, err)
		require.True(t, found)
		assert.True(t, model.Installed)
		assert.Equal(t, uint64(89*1024*1024), model.Size)
	})

	t.Run("ei:efficientnet-b4 check script crashes: returns error", func(t *testing.T) {
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
			// no info event → treated as unexpected failure
			return "", 1
		})
		idx := loadHandlersTestIndex(t, cli)

		_, _, err := idx.GetModelByID(context.Background(), "ei:efficientnet-b4")
		require.Error(t, err)
	})

	t.Run("ei-model-990187-1 custom model: always installed, no Docker call", func(t *testing.T) {
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
			t.Fatal("unexpected Docker call for custom model")
			return "", 0
		})
		idx := loadHandlersTestIndex(t, cli)

		model, found, err := idx.GetModelByID(context.Background(), "ei-model-990187-1")
		require.NoError(t, err)
		require.True(t, found)
		assert.True(t, model.Installed)
	})
}
