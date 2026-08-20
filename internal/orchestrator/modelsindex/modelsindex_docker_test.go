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
	"sync/atomic"
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

// listingWith wraps model entries in the envelope /app/list_models.sh prints.
func listingWith(entries ...string) string {
	return `{"event":"info","models":[` + strings.Join(entries, ",") + `]}`
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

	t.Run("ei:efficientnet-b4 not installed: the listing reports it absent", func(t *testing.T) {
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
			return listingWith(`{"id":"ei:efficientnet-b4","installed":false,"model_size_mb":89}`), 0
		})
		idx := loadHandlersTestIndex(t, cli)

		model, err := idx.GetModelByID(t.Context(), "ei:efficientnet-b4")
		require.NoError(t, err)
		require.NotNil(t, model)
		assert.Equal(t, NotInstalledStatus, model.Status)
		assert.Equal(t, uint64(89*1024*1024), model.Size)
	})

	t.Run("ei:efficientnet-b4 installed: size falls back to the declared one", func(t *testing.T) {
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
			return listingWith(`{"id":"ei:efficientnet-b4","installed":true,"model_size_mb":89}`), 0
		})
		idx := loadHandlersTestIndex(t, cli)

		model, err := idx.GetModelByID(t.Context(), "ei:efficientnet-b4")
		require.NoError(t, err)
		require.NotNil(t, model)
		assert.Equal(t, InstalledStatus, model.Status)
		assert.Equal(t, uint64(89*1024*1024), model.Size)
	})

	t.Run("listing fails: returns an error rather than a declared status", func(t *testing.T) {
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
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

// TestGetModelsReportsDownloading covers the listing's "downloading" field. A transfer in
// flight and a model that was never fetched both report installed=false, so without this
// field the two are indistinguishable to every caller.
func TestGetModelsReportsDownloading(t *testing.T) {
	const listingOutput = `{"event":"info","models":[
		{"id":"ei:efficientnet-b4","name":"EfficientNet-B4","handler":"ei-handler","installed":false,"downloading":true,"model_size_mb":89},
		{"id":"piper-tts-en","name":"Piper TTS","handler":"ai-hub-handler","installed":true,"model_size_mb":46}
	]}`

	cli := newFakeDockerClient(func(_ string, cmd []string) (string, int) {
		if len(cmd) > 0 && cmd[0] == "/app/list_models.sh" {
			return listingOutput, 0
		}
		return "", 0
	})

	dir := paths.New("testdata/with-handlers")
	idx, err := Load(platform.Platform{BoardName: "ventunoq"}, dir, paths.New("not-existing-path"), dir.Join("custom-models"), cli, config.Configuration{})
	require.NoError(t, err)

	models := idx.GetModels(t.Context())
	byID := func(id string) *AIModel {
		t.Helper()
		for i := range models {
			if models[i].ID == id {
				return &models[i]
			}
		}
		t.Fatalf("model %q missing from the index", id)
		return nil
	}

	downloading := byID("ei:efficientnet-b4")
	assert.True(t, downloading.Downloading, "downloading must be carried from the listing")
	assert.Equal(t, NotInstalledStatus, downloading.Status, "a download in flight is not installed yet")

	// The field is absent for this entry: it must read as false, not inherit the neighbour.
	installed := byID("piper-tts-en")
	assert.False(t, installed.Downloading)
	assert.Equal(t, InstalledStatus, installed.Status)
}

// TestLookupRunsOneListing pins the reason Lookup exists: callers that query per brick
// would otherwise pay a container start each, which on a board is seconds per brick.
func TestLookupRunsOneListing(t *testing.T) {
	var listings atomic.Int64
	newIndex := func(t *testing.T) *ModelsIndex {
		t.Helper()
		cli := newFakeDockerClient(func(_ string, cmd []string) (string, int) {
			if len(cmd) > 0 && cmd[0] == "/app/list_models.sh" {
				listings.Add(1)
			}
			return listingWith(`{"id":"ei:efficientnet-b4","installed":true,"model_size_mb":89}`), 0
		})
		dir := paths.New("testdata/with-handlers")
		idx, err := Load(platform.Platform{BoardName: "ventunoq"}, dir, paths.New("not-existing-path"), dir.Join("custom-models"), cli, config.Configuration{})
		require.NoError(t, err)
		return idx
	}

	t.Run("three queries share one listing", func(t *testing.T) {
		listings.Store(0)
		lookup := newIndex(t).NewLookup()

		model, err := lookup.ByID(t.Context(), "ei:efficientnet-b4")
		require.NoError(t, err)
		require.NotNil(t, model)

		_, err = lookup.ByBrick(t.Context(), "arduino:image_classification")
		require.NoError(t, err)

		supported, err := lookup.SupportedByBrick(t.Context(), "ei:efficientnet-b4", "arduino:image_classification")
		require.NoError(t, err)
		assert.True(t, supported)

		assert.Equal(t, int64(1), listings.Load())
	})

	t.Run("a declared model needs no listing at all", func(t *testing.T) {
		listings.Store(0)
		lookup := newIndex(t).NewLookup()

		model, err := lookup.ByID(t.Context(), "piper-tts-en")
		require.NoError(t, err)
		require.NotNil(t, model)

		supported, err := lookup.SupportedByBrick(t.Context(), "piper-tts-en", "arduino:tts")
		require.NoError(t, err)
		assert.True(t, supported)

		assert.Zero(t, listings.Load())
	})

	t.Run("each ModelsIndex call takes its own listing", func(t *testing.T) {
		listings.Store(0)
		idx := newIndex(t)

		_, err := idx.GetModelByID(t.Context(), "ei:efficientnet-b4")
		require.NoError(t, err)
		idx.GetModelsByBrick(t.Context(), "arduino:image_classification")

		assert.Equal(t, int64(2), listings.Load())
	})
}
