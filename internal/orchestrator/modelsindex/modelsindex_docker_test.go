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

func (f *fakeDockerClient) ContainerCreate(_ context.Context, cfg *container.Config, _ *container.HostConfig, _ *network.NetworkingConfig, _ *specs.Platform, _ string) (container.CreateResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.idCounter++
	id := fmt.Sprintf("fake-%d", f.idCounter)
	f.pending[id] = &pendingContainer{image: cfg.Image, cmd: cfg.Cmd, env: cfg.Env}
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
		var stdout string
		var exitCode int
		if f.runFuncEnv != nil {
			stdout, exitCode = f.runFuncEnv(p.image, p.cmd, p.env)
		} else {
			stdout, exitCode = f.runFunc(p.image, p.cmd)
		}
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

	t.Run("listing fails: an id nothing declares is absent, not an error", func(t *testing.T) {
		// The listing is the only thing that can find a model no models-list.yaml entry
		// declares, so a listing that did not run has not found it. Reporting that as a
		// failure turns "no such model" into a 500 for every caller asking by id.
		cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
			return "", 1
		})
		idx := loadHandlersTestIndex(t, cli)

		model, err := idx.GetModelByID(t.Context(), "no-such-model-id")
		require.NoError(t, err)
		assert.Nil(t, model)
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
		if len(cmd) > 0 && cmd[0] == listModelsCmd {
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

	// The field is absent for this entry: it must read as false, not inherit the neighbor.
	installed := byID("piper-tts-en")
	assert.False(t, installed.Downloading)
	assert.Equal(t, InstalledStatus, installed.Status)
}

// TestGetModelsReportsTheRecordedSource covers the link a model was downloaded from,
// which nothing else in the listing carries: an ad-hoc id names the repository directory
// and the file, and says nothing about the request that produced them.
func TestGetModelsReportsTheRecordedSource(t *testing.T) {
	const listingOutput = `{"event":"info","models":[
		{"id":"llamacpp:ggml-org/SmolVLM-256M-Instruct-GGUF/SmolVLM-256M-Instruct-Q8_0",
		 "name":"ggml-org/SmolVLM-256M-Instruct-GGUF/SmolVLM-256M-Instruct-Q8_0",
		 "handler":"llamacpp","model_origin":"user","installed":true,
		 "download_metadata":{
			"downloaded_at":"2026-09-02T09:04:32Z",
			"handler":"hf-handler",
			"model_id":"llamacpp:ggml-org/SmolVLM-256M-Instruct-GGUF/SmolVLM-256M-Instruct-Q8_0",
			"model_origin":"user",
			"inputs":{
				"models_repository":"llamacpp",
				"model_directory":"ggml-org/SmolVLM-256M-Instruct-GGUF",
				"model_url":"https://huggingface.co/ggml-org/SmolVLM-256M-Instruct-GGUF/resolve/main/SmolVLM-256M-Instruct-Q8_0.gguf",
				"model_mmproj_url":"https://huggingface.co/ggml-org/SmolVLM-256M-Instruct-GGUF/resolve/main/mmproj-SmolVLM-256M-Instruct-Q8_0.gguf"}}},
		{"id":"ei:efficientnet-b4","name":"EfficientNet-B4","handler":"ei-handler","installed":true,
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

	vision := byID("llamacpp:ggml-org/SmolVLM-256M-Instruct-GGUF/SmolVLM-256M-Instruct-Q8_0")
	require.NotNil(t, vision.Source)
	assert.Equal(t, "https://huggingface.co/ggml-org/SmolVLM-256M-Instruct-GGUF/resolve/main/SmolVLM-256M-Instruct-Q8_0.gguf", vision.Source.ModelURL)
	assert.Equal(t, "https://huggingface.co/ggml-org/SmolVLM-256M-Instruct-GGUF/resolve/main/mmproj-SmolVLM-256M-Instruct-Q8_0.gguf", vision.Source.MmprojURL)
	assert.Equal(t, "2026-09-02T09:04:32Z", vision.Source.DownloadedAt)

	// A declared model's record names project and impulse numbers, not a link, so there
	// is no source to report: models-list.yaml is what describes where it comes from.
	assert.Nil(t, byID("ei:efficientnet-b4").Source)
}

// TestModelForBrickTakesAnEncodedID covers the write path's normalisation: a client that
// read the models endpoint holds a base64url id, and what it names has to be the same
// model, reported under its plain id so the caller can store that instead.
func TestModelForBrickTakesAnEncodedID(t *testing.T) {
	cli := newFakeDockerClient(func(_ string, cmd []string) (string, int) {
		if len(cmd) > 0 && cmd[0] == listModelsCmd {
			return listingWith(`{"id":"ei:efficientnet-b4","installed":true,"model_size_mb":89}`), 0
		}
		return "", 0
	})
	dir := paths.New("testdata/with-handlers")
	idx, err := Load(platform.Platform{BoardName: "ventunoq"}, dir, paths.New("not-existing-path"), dir.Join("custom-models"), cli, config.Configuration{})
	require.NoError(t, err)

	model, err := idx.ModelForBrick(t.Context(), EncodeID("ei:efficientnet-b4"), "arduino:image_classification")
	require.NoError(t, err)
	require.NotNil(t, model)
	assert.Equal(t, "ei:efficientnet-b4", model.ID, "the answer carries the plain id, whatever the question carried")

	// A brick the model does not serve is not a lookup failure, it is simply no match.
	other, err := idx.ModelForBrick(t.Context(), EncodeID("ei:efficientnet-b4"), "arduino:tts")
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

	t.Run("a declared model needs no listing at all", func(t *testing.T) {
		listings.Store(0)
		lookup := newIndex(t).NewLookup()

		model, err := lookup.ByID(t.Context(), "piper-tts-en")
		require.NoError(t, err)
		require.NotNil(t, model)

		supported, err := lookup.ModelForBrick(t.Context(), "piper-tts-en", "arduino:tts")
		require.NoError(t, err)
		assert.NotNil(t, supported)

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

// TestDownloadByURL pins what reaches the container for a model the catalog does not
// declare: the hf-handler's download script, the caller's URL, and models_repository
// fixed to llamacpp - any other value downloads a model the listing cannot see.
func TestDownloadByURL(t *testing.T) {
	var gotCmd []string
	var gotEnv []string
	cli := newFakeDockerClientWithEnv(func(_ string, cmd, env []string) (string, int) {
		if len(cmd) > 0 && strings.Contains(cmd[0], "hf_model_downloader.sh") {
			gotCmd, gotEnv = cmd, env
		}
		return `{"event":"info","description":"Downloaded to: /models/org/repo","artifacts":["/models/org/repo/m-Q4_0.gguf"],"model_id":"llamacpp:org/repo/m-Q4_0","size_mb":1}` + "\n", 0
	})
	dir := paths.New("testdata/with-handlers")
	idx, err := Load(platform.Platform{BoardName: "ventunoq"}, dir, paths.New("not-existing-path"), dir.Join("custom-models"), cli, config.Configuration{})
	require.NoError(t, err)

	var downloaded *DownloadedModel
	err = idx.DownloadByURL(t.Context(), cli, "llamacpp:org/repo:Q4_0", "", platform.Platform{BoardName: "ventunoq"}, func(e StreamMessage) {
		if m := e.GetModel(); m != nil {
			downloaded = m
		}
	})
	require.NoError(t, err)

	require.NotEmpty(t, gotCmd, "the hf-handler download action must run")
	assert.Contains(t, gotEnv, "model_url=llamacpp:org/repo:Q4_0")
	assert.Contains(t, gotEnv, "models_repository=llamacpp")
	assert.NotContains(t, strings.Join(gotEnv, " "), "model_mmproj_url", "an empty mmproj url must not be passed")

	// The stream carries the model back, so the caller can report what it just created
	// without reading it in again.
	require.NotNil(t, downloaded)
	assert.Equal(t, "llamacpp:org/repo/m-Q4_0", downloaded.ID)
	assert.Equal(t, uint64(1024*1024), downloaded.Size)
}

// A repository already on disk is not transferred again: the handler reports the model it
// finds and exits, with no "complete" event and no progress. The install route answers
// from this event alone, so the model must still come back.
func TestDownloadByURLReportsAnInstalledModel(t *testing.T) {
	cli := newFakeDockerClient(func(_ string, _ []string) (string, int) {
		return `{"event":"info","description":"Model exists: org/repo (m-Q4_0.gguf)","artifacts":["/models/org/repo/m-Q4_0.gguf"],"model_id":"llamacpp:org/repo/m-Q4_0","size_mb":1}` + "\n", 0
	})
	dir := paths.New("testdata/with-handlers")
	idx, err := Load(platform.Platform{BoardName: "ventunoq"}, dir, paths.New("not-existing-path"), dir.Join("custom-models"), cli, config.Configuration{})
	require.NoError(t, err)

	var downloaded *DownloadedModel
	var messages []string
	err = idx.DownloadByURL(t.Context(), cli, "llamacpp:org/repo:Q4_0", "", platform.Platform{BoardName: "ventunoq"}, func(e StreamMessage) {
		if m := e.GetModel(); m != nil {
			downloaded = m
		}
		if e.IsData() {
			messages = append(messages, e.GetData())
		}
	})
	require.NoError(t, err)

	require.NotNil(t, downloaded)
	assert.Equal(t, "llamacpp:org/repo/m-Q4_0", downloaded.ID)
	assert.Equal(t, uint64(1024*1024), downloaded.Size, "the size is the one on disk, not a transfer total")
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

	preLoaded, ok := idx.DeclaredByID("piper-tts-en")
	require.True(t, ok)

	err = idx.Download(t.Context(), cli, *preLoaded, platform.Platform{BoardName: "ventunoq"}, func(StreamMessage) {})

	require.ErrorIs(t, err, ErrNoHandler)
	assert.Zero(t, started, "a pre-loaded model must not start the downloader")
}
