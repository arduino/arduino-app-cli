// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package modelsindex

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

// fakeDockerClient intercepts ContainerCreate/Wait/Attach/Start.
// All other client.APIClient methods panic — they must not be called.
type fakeDockerClient struct {
	client.APIClient

	runFunc func(image string, cmd []string) (stdout string, exitCode int)

	mu        sync.Mutex
	idCounter int
	pending   map[string]*pendingContainer
}

type pendingContainer struct {
	image      string
	cmd        []string
	attachConn net.Conn
	statusCh   chan container.WaitResponse
	errCh      chan error
}

func newFakeDockerClient(runFunc func(image string, cmd []string) (stdout string, exitCode int)) *fakeDockerClient {
	return &fakeDockerClient{
		runFunc: runFunc,
		pending: make(map[string]*pendingContainer),
	}
}

func (f *fakeDockerClient) ContainerCreate(_ context.Context, cfg *container.Config, _ *container.HostConfig, _ *network.NetworkingConfig, _ *specs.Platform, _ string) (container.CreateResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.idCounter++
	id := fmt.Sprintf("fake-%d", f.idCounter)
	f.pending[id] = &pendingContainer{image: cfg.Image, cmd: cfg.Cmd}
	return container.CreateResponse{ID: id}, nil
}

func (f *fakeDockerClient) ContainerWait(_ context.Context, id string, _ container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
	statusCh := make(chan container.WaitResponse, 1)
	errCh := make(chan error, 1)
	f.mu.Lock()
	f.pending[id].statusCh = statusCh
	f.pending[id].errCh = errCh
	f.mu.Unlock()
	return statusCh, errCh
}

func (f *fakeDockerClient) ContainerAttach(_ context.Context, id string, _ container.AttachOptions) (dockertypes.HijackedResponse, error) {
	clientConn, serverConn := net.Pipe()
	f.mu.Lock()
	f.pending[id].attachConn = serverConn
	f.mu.Unlock()
	return dockertypes.HijackedResponse{
		Conn:   clientConn,
		Reader: bufio.NewReader(clientConn),
	}, nil
}

func (f *fakeDockerClient) ContainerStart(_ context.Context, id string, _ container.StartOptions) error {
	f.mu.Lock()
	p := f.pending[id]
	delete(f.pending, id)
	f.mu.Unlock()

	go func() {
		stdout, exitCode := f.runFunc(p.image, p.cmd)
		if stdout != "" {
			w := stdcopy.NewStdWriter(p.attachConn, stdcopy.Stdout)
			fmt.Fprint(w, stdout)
		}
		p.attachConn.Close()
		p.statusCh <- container.WaitResponse{StatusCode: int64(exitCode)}
	}()
	return nil
}

func (f *fakeDockerClient) ContainerRemove(_ context.Context, _ string, _ container.RemoveOptions) error {
	return nil
}

func (f *fakeDockerClient) ImagePull(_ context.Context, _ string, _ image.PullOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (f *fakeDockerClient) ImageInspect(ctx context.Context, _ string, _ ...client.ImageInspectOption) (image.InspectResponse, error) {
	return image.InspectResponse{}, nil
}

func TestGetModelByID_WithDockerMock(t *testing.T) {
	loadHandlersTestIndex := func(t *testing.T, dockerCli client.APIClient) *ModelsIndex {
		t.Helper()
		dir := paths.New("testdata/with-handlers")
		customModelsDir := dir.Join("custom-models")
		idx, err := Load(platform.Platform{BoardName: "ventunoq"}, dir, paths.New("not-existing-path"), customModelsDir, dockerCli, config.Configuration{})
		require.NoError(t, err)
		return idx
	}

	t.Run("the custom modeldir volume is not resolved at load time", func(t *testing.T) {
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
			return "", 0
		})
		idx := loadHandlersTestIndex(t, cli)

		require.Equal(t, []string{"${MODELS_PATH}:/models"}, idx.Handlers.listing.Volumes)
		h, ok := idx.Handlers.GetHandlerByID("ai-hub-handler")
		require.True(t, ok)
		require.Equal(t, []string{"${MODELS_PATH:-/var/lib/arduino-app-cli/models}:/models"}, h.Volumes)

	})

	t.Run("piper-tts-en is pre-loaded: returns size from metadata, no Docker call", func(t *testing.T) {
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
			t.Fatal("unexpected Docker call for pre-loaded model")
			return "", 0
		})
		idx := loadHandlersTestIndex(t, cli)

		model, err := idx.GetModelByID(t.Context(), "piper-tts-en")
		require.NoError(t, err)
		require.NotNil(t, model)
		assert.Equal(t, uint64(46*1024*1024), model.Size)
	})

	t.Run("ei:efficientnet-b4 not installed: check exits 1 with error event", func(t *testing.T) {
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
			// check action → model not present (script signals this via error event + exit 0)
			return "{\"event\":\"error\",\"description\":\"model not installed\"}\n", 0
		})
		idx := loadHandlersTestIndex(t, cli)

		model, err := idx.GetModelByID(t.Context(), "ei:efficientnet-b4")
		require.NoError(t, err)
		require.NotNil(t, model)
		assert.Equal(t, NotInstalledStatus, model.Status)
		assert.Equal(t, uint64(89*1024*1024), model.Size)
	})

	t.Run("ei:efficientnet-b4 installed: check exits 0, size from metadata", func(t *testing.T) {
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
			// check action → model is present (script signals this via info event + exit 0)
			return "{\"event\":\"info\"}\n", 0
		})
		idx := loadHandlersTestIndex(t, cli)

		model, err := idx.GetModelByID(t.Context(), "ei:efficientnet-b4")
		require.NoError(t, err)
		require.NotNil(t, model)
		assert.Equal(t, InstalledStatus, model.Status)
		assert.Equal(t, uint64(89*1024*1024), model.Size)
	})

	t.Run("ei:efficientnet-b4 check script crashes: returns error", func(t *testing.T) {
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
			// no info event → treated as unexpected failure
			return "", 1
		})
		idx := loadHandlersTestIndex(t, cli)

		_, err := idx.GetModelByID(t.Context(), "ei:efficientnet-b4")
		require.Error(t, err)
	})

	t.Run("ei-model-990187-1 custom model: always installed, no Docker call", func(t *testing.T) {
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
			t.Fatal("unexpected Docker call for custom model")
			return "", 0
		})
		idx := loadHandlersTestIndex(t, cli)

		model, err := idx.GetModelByID(t.Context(), "ei-model-990187-1")
		require.NoError(t, err)
		require.NotNil(t, model)
		assert.Equal(t, InstalledStatus, model.Status)
	})
}
