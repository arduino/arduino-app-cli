// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package modelsindex

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"iter"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/client"
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
	// runFuncEnv, when set, is called instead of runFunc and also receives the
	// container's environment.
	runFuncEnv func(image string, cmd, env []string) (stdout string, exitCode int)

	mu        sync.Mutex
	idCounter int
	pending   map[string]*pendingContainer
}

type pendingContainer struct {
	image      string
	cmd        []string
	env        []string
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

func newFakeDockerClientWithEnv(runFunc func(image string, cmd, env []string) (stdout string, exitCode int)) *fakeDockerClient {
	return &fakeDockerClient{
		runFuncEnv: runFunc,
		pending:    make(map[string]*pendingContainer),
	}
}

func (f *fakeDockerClient) ContainerCreate(_ context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.idCounter++
	id := fmt.Sprintf("fake-%d", f.idCounter)
	cfg := options.Config
	f.pending[id] = &pendingContainer{image: cfg.Image, cmd: cfg.Cmd, env: cfg.Env}
	return client.ContainerCreateResult{ID: id}, nil
}

func (f *fakeDockerClient) ContainerWait(_ context.Context, id string, _ client.ContainerWaitOptions) client.ContainerWaitResult {
	statusCh := make(chan container.WaitResponse, 1)
	errCh := make(chan error, 1)
	f.mu.Lock()
	f.pending[id].statusCh = statusCh
	f.pending[id].errCh = errCh
	f.mu.Unlock()
	return client.ContainerWaitResult{Result: statusCh, Error: errCh}
}

func (f *fakeDockerClient) ContainerAttach(_ context.Context, id string, _ client.ContainerAttachOptions) (client.ContainerAttachResult, error) {
	clientConn, serverConn := net.Pipe()
	f.mu.Lock()
	f.pending[id].attachConn = serverConn
	f.mu.Unlock()
	return client.ContainerAttachResult{HijackedResponse: client.HijackedResponse{
		Conn:   clientConn,
		Reader: bufio.NewReader(clientConn),
	}}, nil
}

func (f *fakeDockerClient) ContainerStart(_ context.Context, id string, _ client.ContainerStartOptions) (client.ContainerStartResult, error) {
	f.mu.Lock()
	p := f.pending[id]
	delete(f.pending, id)
	f.mu.Unlock()

	go func() {
		var stdout string
		var exitCode int
		if f.runFuncEnv != nil {
			stdout, exitCode = f.runFuncEnv(p.image, p.cmd, p.env)
		} else {
			stdout, exitCode = f.runFunc(p.image, p.cmd)
		}
		if stdout != "" {
			writeStdoutFrame(p.attachConn, stdout)
		}
		p.attachConn.Close()
		p.statusCh <- container.WaitResponse{StatusCode: int64(exitCode)}
	}()
	return client.ContainerStartResult{}, nil
}

func (f *fakeDockerClient) ContainerRemove(_ context.Context, _ string, _ client.ContainerRemoveOptions) (client.ContainerRemoveResult, error) {
	return client.ContainerRemoveResult{}, nil
}

func (f *fakeDockerClient) ImagePull(_ context.Context, _ string, _ client.ImagePullOptions) (client.ImagePullResponse, error) {
	return fakePullResponse{ReadCloser: io.NopCloser(strings.NewReader(""))}, nil
}

// fakePullResponse is what the client returns for a pull: a stream, plus the two ways
// of reading it the api offers.
type fakePullResponse struct {
	io.ReadCloser
}

func (fakePullResponse) JSONMessages(context.Context) iter.Seq2[jsonstream.Message, error] {
	return func(func(jsonstream.Message, error) bool) {}
}

func (fakePullResponse) Wait(context.Context) error { return nil }

func (f *fakeDockerClient) ImageInspect(_ context.Context, _ string, _ ...client.ImageInspectOption) (client.ImageInspectResult, error) {
	return client.ImageInspectResult{}, nil
}

// writeStdoutFrame multiplexes the payload the way the daemon does, which is what
// stdcopy.StdCopy unframes on the other side.
func writeStdoutFrame(w io.Writer, payload string) {
	header := make([]byte, 8)
	header[0] = byte(stdcopy.Stdout)
	binary.BigEndian.PutUint32(header[4:], uint32(len(payload))) // nolint:gosec // a test payload is small
	_, _ = w.Write(header)
	_, _ = io.WriteString(w, payload)
}

// listModelsCmd is the listing container's command, as testdata/with-handlers declares it.
const listModelsCmd = "/app/list_models.sh"

// listingWith wraps model entries in the envelope the listing command prints.
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

	t.Run("piper-tts-en is pre-loaded: state comes from the listing", func(t *testing.T) {
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
			return listingWith(`{"id":"piper-tts-en","installed":true,"model_size_mb":46}`), 0
		})
		idx := loadHandlersTestIndex(t, cli)

		model, err := idx.NewLookup().ByID(t.Context(), "piper-tts-en")
		require.NoError(t, err)
		require.NotNil(t, model)
		assert.Equal(t, InstalledStatus, model.Status)
		assert.Equal(t, uint64(46*1024*1024), model.SizeBytes)
	})

	t.Run("piper-tts-en is pre-loaded: files missing reads not installed", func(t *testing.T) {
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
			return listingWith(`{"id":"piper-tts-en","installed":false,"model_size_mb":46}`), 0
		})
		idx := loadHandlersTestIndex(t, cli)

		model, err := idx.NewLookup().ByID(t.Context(), "piper-tts-en")
		require.NoError(t, err)
		require.NotNil(t, model)
		assert.Equal(t, NotInstalledStatus, model.Status)
	})

	t.Run("ei:efficientnet-b4 not installed: the listing reports it absent", func(t *testing.T) {
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
			return listingWith(`{"id":"ei:efficientnet-b4","installed":false,"model_size_mb":89}`), 0
		})
		idx := loadHandlersTestIndex(t, cli)

		model, err := idx.NewLookup().ByID(t.Context(), "ei:efficientnet-b4")
		require.NoError(t, err)
		require.NotNil(t, model)
		assert.Equal(t, NotInstalledStatus, model.Status)
		assert.Equal(t, uint64(89*1024*1024), model.SizeBytes)
	})

	t.Run("ei:efficientnet-b4 installed: size falls back to the declared one", func(t *testing.T) {
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
			return listingWith(`{"id":"ei:efficientnet-b4","installed":true,"model_size_mb":89}`), 0
		})
		idx := loadHandlersTestIndex(t, cli)

		model, err := idx.NewLookup().ByID(t.Context(), "ei:efficientnet-b4")
		require.NoError(t, err)
		require.NotNil(t, model)
		assert.Equal(t, InstalledStatus, model.Status)
		assert.Equal(t, uint64(89*1024*1024), model.SizeBytes)
	})

	t.Run("listing fails: returns an error rather than a declared status", func(t *testing.T) {
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
			return "", 1
		})
		idx := loadHandlersTestIndex(t, cli)

		_, err := idx.NewLookup().ByID(t.Context(), "ei:efficientnet-b4")
		require.Error(t, err)
	})

	t.Run("listing fails: an id nothing declares is an error too", func(t *testing.T) {
		// Only the listing can find an undeclared model, so a listing that did not run cannot
		// say it is absent: a 404 here would hide the broken listing.
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
			return "", 1
		})
		idx := loadHandlersTestIndex(t, cli)

		_, err := idx.NewLookup().ByID(t.Context(), "no-such-model-id")
		require.Error(t, err)
	})

	t.Run("ei-model-990187-1 custom model: installed, found by the folder scan", func(t *testing.T) {
		// The listing runs, but knows nothing of custom models: the folder scan finds it.
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
			return listingWith(), 0
		})
		idx := loadHandlersTestIndex(t, cli)

		model, err := idx.NewLookup().ByID(t.Context(), "ei-model-990187-1")
		require.NoError(t, err)
		require.NotNil(t, model)
		assert.Equal(t, InstalledStatus, model.Status)
	})
}

// TestGetModelsMergesTheListing covers what the listing adds to a model the index knows:
// the transfer in flight, and the link the record kept.
func TestGetModelsMergesTheListing(t *testing.T) {
	t.Run("a transfer in flight is its own status", func(t *testing.T) {
		const listingOutput = `{"event":"info","models":[
			{"id":"ei:efficientnet-b4","name":"EfficientNet-B4","handler":"ei-handler","installed":false,"downloading":true,"model_size_mb":89},
			{"id":"piper-tts-en","name":"Piper TTS","handler":"ai-hub-handler","installed":true,"model_size_mb":46}
		]}`

		cli := newFakeDockerClient(func(_ string, cmd []string) (string, int) {
			if len(cmd) > 0 && cmd[0] == listModelsCmd {
				return listingOutput, 0
			}
			return "", 0
		})

		dir := paths.New("testdata/with-handlers")
		idx, err := Load(platform.Platform{BoardName: "ventunoq"}, dir, paths.New("not-existing-path"), dir.Join("custom-models"), cli, config.Configuration{})
		require.NoError(t, err)

		models, err := idx.NewLookup().All(t.Context())
		require.NoError(t, err)
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
		assert.Equal(t, DownloadingStatus, downloading.Status, "a transfer in flight is its own status")

		// The field is absent for this entry: it must not inherit the neighbor's.
		installed := byID("piper-tts-en")
		assert.Equal(t, InstalledStatus, installed.Status)
	})

	t.Run("the record carries the link a model was downloaded from", func(t *testing.T) {
		const listingOutput = `{"event":"info","models":[
			{"id":"llamacpp:ggml-org/SmolVLM-256M-Instruct-GGUF/SmolVLM-256M-Instruct-Q8_0",
			 "name":"ggml-org/SmolVLM-256M-Instruct-GGUF/SmolVLM-256M-Instruct-Q8_0",
			 "handler":"hf-handler","runtime":"llamacpp","model_publisher":"ggml-org",
			 "model_origin":"user","installed":true,
			 "mmproj":"/models/llamacpp/ggml-org/SmolVLM-256M-Instruct-GGUF/mmproj-SmolVLM-256M-Instruct-Q8_0.gguf",
			 "download_metadata":{
				"downloaded_at":"2026-09-02T09:04:32Z",
				"handler":"hf-handler",
				"model_id":"llamacpp:ggml-org/SmolVLM-256M-Instruct-GGUF/SmolVLM-256M-Instruct-Q8_0",
				"model_origin":"user",
				"inputs":{
					"models_repository":"llamacpp",
					"model_directory":"ggml-org/SmolVLM-256M-Instruct-GGUF",
					"model_url":"https://huggingface.co/ggml-org/SmolVLM-256M-Instruct-GGUF/resolve/main/SmolVLM-256M-Instruct-Q8_0.gguf"}}},
			{"id":"ei:efficientnet-b4","name":"EfficientNet-B4","handler":"ei-handler","installed":true,
			 "runtime":"edge-impulse-sdk","model_publisher":"qualcomm-ai-hub",
			 "download_metadata":{
				"downloaded_at":"2026-08-30T11:02:00Z",
				"handler":"ei-handler",
				"model_id":"ei:efficientnet-b4",
				"model_origin":"builtin",
				"inputs":{"ei_project_id":"948887","ei_impulse_id":"4"}}}
		]}`

		cli := newFakeDockerClient(func(_ string, cmd []string) (string, int) {
			if len(cmd) > 0 && cmd[0] == listModelsCmd {
				return listingOutput, 0
			}
			return "", 0
		})

		dir := paths.New("testdata/with-handlers")
		idx, err := Load(platform.Platform{BoardName: "ventunoq"}, dir, paths.New("not-existing-path"), dir.Join("custom-models"), cli, config.Configuration{})
		require.NoError(t, err)

		models, err := idx.NewLookup().All(t.Context())
		require.NoError(t, err)
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

		// The listing reports a projection file, so the vlm brick is the one that can run it.
		// Nothing declares this model, so its bricks are derived from what was downloaded.
		vision := byID("llamacpp:ggml-org/SmolVLM-256M-Instruct-GGUF/SmolVLM-256M-Instruct-Q8_0")
		assert.Equal(t, []BrickConfig{{ID: vlmBrickID}}, vision.Bricks)
		assert.Equal(t, map[string]string{
			"source-model-url": "https://huggingface.co/ggml-org/SmolVLM-256M-Instruct-GGUF/resolve/main/SmolVLM-256M-Instruct-Q8_0.gguf",
			"runtime":          "llamacpp",
			"publisher":        "ggml-org",
		}, vision.Metadata, "the link comes from the record the listing carries")

		// This record names project and impulse numbers, not a link, so no link is added.
		assert.Equal(t, map[string]string{
			"model_size_mb": "89",
			"source":        "edgeimpulse",
			"runtime":       "edge-impulse-sdk",
			"publisher":     "qualcomm-ai-hub",
		}, byID("ei:efficientnet-b4").Metadata)

		known, ok := idx.known("ei:efficientnet-b4")
		require.True(t, ok)
		assert.Equal(t, map[string]string{"model_size_mb": "89", "source": "edgeimpulse"}, known.Metadata,
			"the index's own entry stays as declared")
	})
}

// TestModelForBrick covers the write path: the lookup answers on plain ids, and reports
// the model under its own id so the caller stores that.
func TestModelForBrick(t *testing.T) {
	cli := newFakeDockerClient(func(_ string, cmd []string) (string, int) {
		if len(cmd) > 0 && cmd[0] == listModelsCmd {
			return listingWith(`{"id":"ei:efficientnet-b4","installed":true,"model_size_mb":89}`), 0
		}
		return "", 0
	})
	dir := paths.New("testdata/with-handlers")
	idx, err := Load(platform.Platform{BoardName: "ventunoq"}, dir, paths.New("not-existing-path"), dir.Join("custom-models"), cli, config.Configuration{})
	require.NoError(t, err)

	model, err := idx.NewLookup().ModelForBrick(t.Context(), "ei:efficientnet-b4", "arduino:image_classification")
	require.NoError(t, err)
	require.NotNil(t, model)
	assert.Equal(t, "ei:efficientnet-b4", model.ID)

	// A brick the model does not serve is not a lookup failure, it is simply no match.
	other, err := idx.NewLookup().ModelForBrick(t.Context(), "ei:efficientnet-b4", "arduino:tts")
	require.NoError(t, err)
	assert.Nil(t, other)
}

// TestLookupRunsOneListing pins the reason Lookup exists: callers that query per brick
// would otherwise pay a container start each, which on a board is seconds per brick.
func TestLookupRunsOneListing(t *testing.T) {
	var listings atomic.Int64
	newIndex := func(t *testing.T) *ModelsIndex {
		t.Helper()
		cli := newFakeDockerClient(func(_ string, cmd []string) (string, int) {
			if len(cmd) > 0 && cmd[0] == listModelsCmd {
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

		supported, err := lookup.ModelForBrick(t.Context(), "ei:efficientnet-b4", "arduino:image_classification")
		require.NoError(t, err)
		assert.NotNil(t, supported)

		assert.Equal(t, int64(1), listings.Load())
	})

	t.Run("a pre-loaded model is listed like any other", func(t *testing.T) {
		listings.Store(0)
		lookup := newIndex(t).NewLookup()

		model, err := lookup.ByID(t.Context(), "piper-tts-en")
		require.NoError(t, err)
		require.NotNil(t, model)

		supported, err := lookup.ModelForBrick(t.Context(), "piper-tts-en", "arduino:tts")
		require.NoError(t, err)
		assert.NotNil(t, supported)

		assert.Equal(t, int64(1), listings.Load())
	})

	t.Run("new Lookups share the cached listing", func(t *testing.T) {
		listings.Store(0)
		idx := newIndex(t)

		_, err := idx.NewLookup().ByID(t.Context(), "ei:efficientnet-b4")
		require.NoError(t, err)
		_, err = idx.NewLookup().ByBrick(t.Context(), "arduino:image_classification")
		require.NoError(t, err)

		assert.Equal(t, int64(1), listings.Load())
	})

	t.Run("Refresh runs the listing again", func(t *testing.T) {
		listings.Store(0)
		idx := newIndex(t)

		_, err := idx.NewLookup().ByID(t.Context(), "ei:efficientnet-b4")
		require.NoError(t, err)
		_, err = idx.Refresh(t.Context())
		require.NoError(t, err)
		_, err = idx.NewLookup().ByID(t.Context(), "ei:efficientnet-b4")
		require.NoError(t, err)

		assert.Equal(t, int64(2), listings.Load())
	})

	t.Run("a failed listing is remembered per Lookup", func(t *testing.T) {
		var failed atomic.Int64
		cli := newFakeDockerClient(func(_ string, cmd []string) (string, int) {
			if len(cmd) > 0 && cmd[0] == listModelsCmd {
				failed.Add(1)
			}
			return "", 1
		})
		dir := paths.New("testdata/with-handlers")
		idx, err := Load(platform.Platform{BoardName: "ventunoq"}, dir, paths.New("not-existing-path"), dir.Join("custom-models"), cli, config.Configuration{})
		require.NoError(t, err)

		lookup := idx.NewLookup()
		_, err1 := lookup.ByID(t.Context(), "ei:efficientnet-b4")
		_, err2 := lookup.ByID(t.Context(), "piper-tts-en")
		_, err3 := lookup.All(t.Context())
		require.Error(t, err1)
		require.Error(t, err2)
		require.Error(t, err3)
		assert.Equal(t, int64(1), failed.Load(), "one Lookup, one attempt")

		// A failure is not cached in the index: the next Lookup tries again.
		_, err = idx.NewLookup().ByID(t.Context(), "ei:efficientnet-b4")
		require.Error(t, err)
		assert.Equal(t, int64(2), failed.Load(), "a new Lookup retries")
	})
}

func TestRefreshEmptyCatalog(t *testing.T) {
	tmp := paths.New(t.TempDir())
	require.NoError(t, tmp.Join("models-list.yaml").WriteFile([]byte("models: []\n")))

	// No handlers file and no Docker client: the dry catalog is the whole answer.
	idx, err := Load(platform.Platform{}, tmp, tmp, nil, nil, config.Configuration{})
	require.NoError(t, err)

	models, err := idx.Refresh(t.Context())
	require.ErrorIs(t, err, ErrEmptyCatalog)
	assert.Nil(t, models)
	assert.Nil(t, idx.snapshot(), "an empty catalog is not cached")
}

// TestDownloadByURL pins what reaches the container for an undeclared model: the
// hf-handler's script, the caller's URL, and models_repository fixed to llamacpp.
// downloadedEntry is what the listing reports for the model these downloads write: the
// record names the link, so the reconcile step can describe it.
const downloadedEntry = `{"id":"llamacpp:org/repo/m-Q4_0","name":"org/repo/m-Q4_0","handler":"llamacpp",
	"model_origin":"user","installed":true,"disk_size_mb":1,
	"download_metadata":{"handler":"hf-handler","model_id":"llamacpp:org/repo/m-Q4_0",
		"inputs":{"models_repository":"llamacpp","model_url":"llamacpp:org/repo:Q4_0"}}}`

func TestDownloadByURL(t *testing.T) {
	var gotCmd []string
	var gotEnv []string
	cli := newFakeDockerClientWithEnv(func(_ string, cmd, env []string) (string, int) {
		if len(cmd) > 0 && cmd[0] == listModelsCmd {
			return listingWith(downloadedEntry), 0
		}
		if len(cmd) > 0 && strings.Contains(cmd[0], "hf_model_downloader.sh") {
			gotCmd, gotEnv = cmd, env
		}
		return `{"event":"info","description":"Downloaded to: /models/org/repo","artifacts":["/models/org/repo/m-Q4_0.gguf"],"model_id":"llamacpp:org/repo/m-Q4_0","size_mb":1}` + "\n", 0
	})
	dir := paths.New("testdata/with-handlers")
	idx, err := Load(platform.Platform{BoardName: "ventunoq"}, dir, paths.New("not-existing-path"), dir.Join("custom-models"), cli, config.Configuration{})
	require.NoError(t, err)

	installed, err := idx.DownloadByURL(t.Context(), cli, "llamacpp:org/repo:Q4_0", "", platform.Platform{BoardName: "ventunoq"}, func(StreamMessage) {})
	require.NoError(t, err)

	require.NotEmpty(t, gotCmd, "the hf-handler download action must run")
	assert.Contains(t, gotEnv, "model_url=llamacpp:org/repo:Q4_0")
	assert.Contains(t, gotEnv, "models_repository=llamacpp")
	assert.NotContains(t, strings.Join(gotEnv, " "), "model_mmproj_url", "an empty mmproj url must not be passed")

	// The answer is the listed model, so the caller reports what a later GetModels reports.
	assert.Equal(t, "llamacpp:org/repo/m-Q4_0", installed.ID)
	assert.Equal(t, InstalledStatus, installed.Status)
	assert.Equal(t, uint64(1024*1024), installed.SizeBytes)
	assert.Equal(t, map[string]string{"source-model-url": "llamacpp:org/repo:Q4_0"}, installed.Metadata)
}

// A repository already on disk is not transferred again: the handler reports the model it
// finds, with no "complete" event, and the route answers from that event alone.
func TestDownloadByURLReportsAnInstalledModel(t *testing.T) {
	cli := newFakeDockerClient(func(_ string, cmd []string) (string, int) {
		if len(cmd) > 0 && cmd[0] == listModelsCmd {
			return listingWith(downloadedEntry), 0
		}
		return `{"event":"info","description":"Model exists: org/repo (m-Q4_0.gguf)","artifacts":["/models/org/repo/m-Q4_0.gguf"],"model_id":"llamacpp:org/repo/m-Q4_0","size_mb":1}` + "\n", 0
	})
	dir := paths.New("testdata/with-handlers")
	idx, err := Load(platform.Platform{BoardName: "ventunoq"}, dir, paths.New("not-existing-path"), dir.Join("custom-models"), cli, config.Configuration{})
	require.NoError(t, err)

	var messages []string
	installed, err := idx.DownloadByURL(t.Context(), cli, "llamacpp:org/repo:Q4_0", "", platform.Platform{BoardName: "ventunoq"}, func(e StreamMessage) {
		if e.IsData() {
			messages = append(messages, e.GetData())
		}
	})
	require.NoError(t, err)

	assert.Equal(t, "llamacpp:org/repo/m-Q4_0", installed.ID)
	assert.Equal(t, uint64(1024*1024), installed.SizeBytes, "the size is the one on disk, not a transfer total")
	assert.Equal(t, []string{"Model exists: org/repo (m-Q4_0.gguf)"}, messages)
}

// A model installed by its declaration reaches no container. The install route answers it
// without calling Download at all, so this guards the other callers.
func TestDownloadRefusesAModelWithNothingToDownload(t *testing.T) {
	var started int
	cli := newFakeDockerClient(func(_ string, _ []string) (string, int) {
		started++
		return "", 0
	})
	dir := paths.New("testdata/with-handlers")
	idx, err := Load(platform.Platform{BoardName: "ventunoq"}, dir, paths.New("not-existing-path"), dir.Join("custom-models"), cli, config.Configuration{})
	require.NoError(t, err)

	// A nil docker client: a model that needs no download must not read it.
	installed, err := idx.Install(t.Context(), nil, "piper-tts-en", platform.Platform{BoardName: "ventunoq"}, func(StreamMessage) {})

	require.NoError(t, err)
	assert.Equal(t, InstalledStatus, installed.Status, "a pre-loaded model is installed already")
	assert.Zero(t, started, "a pre-loaded model must not start the downloader")

	// The guard stays on the runner, for a caller that reaches it with such a model.
	preLoaded, ok := idx.known("piper-tts-en")
	require.True(t, ok)
	_, err = idx.runDownload(t.Context(), cli, *preLoaded, platform.Platform{BoardName: "ventunoq"}, func(StreamMessage) {})
	require.ErrorIs(t, err, ErrNoHandler)
}

func TestLockKey(t *testing.T) {
	const board = "ventunoq"
	userHF := AIModel{
		ID:     "llamacpp:org/repo/m-Q4_0",
		Origin: UserOrigin,
		Deployment: &ModelDeployment{
			Handler:   "hf-handler",
			Variables: []map[string]PlatformDeploymentConfig{{board: {Variables: map[string]string{"model_url": "https://hf.co/org/repo/m-Q4_0.gguf"}}}},
		},
	}
	userNoURL := userHF
	userNoURL.Deployment = &ModelDeployment{
		Handler:   "hf-handler",
		Variables: []map[string]PlatformDeploymentConfig{{board: {Variables: map[string]string{"models_repository": "llamacpp"}}}},
	}

	tests := []struct {
		name  string
		model AIModel
		want  string
	}{
		{"curated: its id", AIModel{ID: "gemma-3-1b", Origin: CuratedOrigin, Deployment: &ModelDeployment{Handler: "hf-handler"}}, "gemma-3-1b"},
		{"user EI, no deployment: its id", AIModel{ID: "ei-model-1-2", Origin: UserOrigin}, "ei-model-1-2"},
		{"user HF: the link its install locked", userHF, "https://hf.co/org/repo/m-Q4_0.gguf"},
		{"user model with no link: its id", userNoURL, "llamacpp:org/repo/m-Q4_0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, lockKey(tc.model, board))
		})
	}
}

// TestDownloadByURLHoldsTheLock: PrecheckDownload takes the link's lock and the caller holds
// it through the download. A second precheck of the same link, or a delete of the model the
// link installs, is refused meanwhile; once released, the link is free again.
func TestDownloadByURLHoldsTheLock(t *testing.T) {
	const url = "llamacpp:org/repo:Q4_0" // the link downloadedEntry records
	var downloads atomic.Int64
	started := make(chan struct{})
	release := make(chan struct{})
	cli := newFakeDockerClient(func(_ string, cmd []string) (string, int) {
		switch {
		case len(cmd) > 0 && cmd[0] == listModelsCmd:
			return listingWith(downloadedEntry), 0
		case len(cmd) > 0 && strings.Contains(cmd[0], "hf_model_info.sh"):
			return `{"event":"stat","size_bytes":1}` + "\n", 0
		case len(cmd) > 0 && strings.Contains(cmd[0], "hf_model_checker.sh"):
			return `{"event":"error","description":"Model does not exist","downloading":false}` + "\n", 1
		}
		if downloads.Add(1) == 1 {
			close(started)
			<-release // hold the first download until the test is done checking
		}
		return `{"event":"info","description":"done","model_id":"llamacpp:org/repo/m-Q4_0","size_mb":1}` + "\n", 0
	})
	dir := paths.New("testdata/with-handlers")
	plat := platform.Platform{BoardName: "ventunoq"}
	idx, err := Load(plat, dir, paths.New(t.TempDir()), dir.Join("custom-models"), cli, config.Configuration{})
	require.NoError(t, err)
	idx.locksDir = paths.New(t.TempDir()) // a test config has no data dir: locking would be off

	firstErr := make(chan error, 1)
	go func() {
		unlock, res, err := idx.PrecheckDownload(context.Background(), cli, url, "", plat)
		if err != nil {
			firstErr <- err
			return
		}
		defer unlock()
		if res.Installed {
			firstErr <- errors.New("precheck reported the model installed")
			return
		}
		_, err = idx.DownloadByURL(context.Background(), cli, url, "", plat, func(StreamMessage) {})
		firstErr <- err
	}()
	<-started

	t.Run("a second precheck of the same link is refused", func(t *testing.T) {
		_, _, err := idx.PrecheckDownload(t.Context(), cli, url, "", plat)
		require.ErrorIs(t, err, ErrInstallInProgress)
		assert.Equal(t, int64(1), downloads.Load(), "no second download container")
	})

	t.Run("deleting the model that link installs takes the same lock", func(t *testing.T) {
		listed, err := idx.NewLookup().ByID(t.Context(), "llamacpp:org/repo/m-Q4_0")
		require.NoError(t, err)
		require.NotNil(t, listed)
		unlock, err := lockModel(idx.locksDir, lockKey(*listed, plat.BoardName))
		defer unlock()
		require.ErrorIs(t, err, ErrInstallInProgress)
	})

	close(release)
	require.NoError(t, <-firstErr)

	t.Run("after it ends the link is free again", func(t *testing.T) {
		unlock, err := lockModel(idx.locksDir, url)
		require.NoError(t, err)
		unlock()
	})
}
