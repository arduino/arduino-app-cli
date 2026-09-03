// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/arduino/go-paths-helper"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-app-cli/internal/render"
)

// TestInstalledModel covers the install route's last step: describing what the handler
// just wrote, from the download event and the declaration alone. No listing runs, so
// whatever cannot be answered here cannot be answered at all.
func TestInstalledModel(t *testing.T) {
	const adHocID = "llamacpp:unsloth/SmolLM2-135M-Instruct-GGUF/SmolLM2-135M-Instruct-Q4_K_M"

	t.Run("a source the model list does not declare becomes a user-configured model", func(t *testing.T) {
		idx := &modelsindex.ModelsIndex{}

		model, ok := installedModel(idx, nil, &modelsindex.DownloadedModel{ID: adHocID, Size: 1024},
			&modelsindex.ModelSource{ModelURL: "https://huggingface.co/unsloth/SmolLM2-135M-Instruct-GGUF/resolve/main/SmolLM2-135M-Instruct-Q4_K_M.gguf"})

		require.True(t, ok)
		assert.Equal(t, adHocID, model.ID)
		assert.Equal(t, "unsloth/SmolLM2-135M-Instruct-GGUF/SmolLM2-135M-Instruct-Q4_K_M", model.Name)
		assert.Equal(t, modelsindex.InstalledStatus, model.Status)
		assert.Equal(t, uint64(1024), model.Size)
		assert.False(t, model.IsBuiltIn, "a model the user installed must stay deletable")
		assert.Equal(t, []modelsindex.BrickConfig{{ID: "arduino:llm"}}, model.Bricks,
			"no projection file was fetched, so it is a text model")
	})

	t.Run("a download that fetched a projection file is a vision model", func(t *testing.T) {
		const visionID = "llamacpp:ggml-org/SmolVLM-256M-Instruct-GGUF/SmolVLM-256M-Instruct-Q8_0"

		model, ok := installedModel(&modelsindex.ModelsIndex{}, nil, &modelsindex.DownloadedModel{ID: visionID},
			&modelsindex.ModelSource{
				ModelURL:  "https://huggingface.co/ggml-org/SmolVLM-256M-Instruct-GGUF/resolve/main/SmolVLM-256M-Instruct-Q8_0.gguf",
				MmprojURL: "https://huggingface.co/ggml-org/SmolVLM-256M-Instruct-GGUF/resolve/main/mmproj-SmolVLM-256M-Instruct-Q8_0.gguf",
			})

		require.True(t, ok)
		assert.Equal(t, []modelsindex.BrickConfig{{ID: "arduino:vlm"}}, model.Bricks,
			"the vlm brick is the one that can run it")
	})

	t.Run("a file landing where the model list declares it is that declared model", func(t *testing.T) {
		// The path named a Hugging Face source, so nothing was declared up front, but the
		// handler resolved the id against the catalog and named a declared model. Its
		// entry describes it, rather than the bare id the event carried.
		idx := &modelsindex.ModelsIndex{InternalModels: []modelsindex.AIModel{
			{ID: "llamacpp:gemma-3-1b-it-Q4_0", Name: "Gemma 3 1B", Description: "An efficient AI model."},
		}}

		model, ok := installedModel(idx, nil, &modelsindex.DownloadedModel{ID: "llamacpp:gemma-3-1b-it-Q4_0", Size: 2048}, nil)

		require.True(t, ok)
		assert.Equal(t, "Gemma 3 1B", model.Name)
		assert.Equal(t, "An efficient AI model.", model.Description)
		assert.Equal(t, modelsindex.InstalledStatus, model.Status)
		assert.Equal(t, uint64(2048), model.Size)
	})

	t.Run("a download naming nothing cannot be described", func(t *testing.T) {
		// A models-downloader too old to report model_id: the model installed, but naming
		// it here would promise an id no later request resolves.
		_, ok := installedModel(&modelsindex.ModelsIndex{}, nil, nil, nil)

		assert.False(t, ok)
	})

	t.Run("a declared model takes the size the event reports", func(t *testing.T) {
		declared := &modelsindex.AIModel{ID: "llamacpp:gemma-3-1b-it-Q4_0", Name: "Gemma 3 1B", Size: 1000}

		model, ok := installedModel(&modelsindex.ModelsIndex{}, declared, &modelsindex.DownloadedModel{ID: declared.ID, Size: 2000}, nil)

		require.True(t, ok)
		assert.Equal(t, modelsindex.InstalledStatus, model.Status)
		assert.Equal(t, uint64(2000), model.Size, "the on-disk size is closer than the declared one")
	})

	t.Run("a declared model keeps its declared size when the event carries none", func(t *testing.T) {
		// size_mb is omitted when a file cannot be stat'd. The declaration still holds a
		// model_size_mb, and reporting zero would read as an empty install.
		declared := &modelsindex.AIModel{ID: "llamacpp:gemma-3-1b-it-Q4_0", Name: "Gemma 3 1B", Size: 1000}

		model, ok := installedModel(&modelsindex.ModelsIndex{}, declared, &modelsindex.DownloadedModel{ID: declared.ID}, nil)

		require.True(t, ok)
		assert.Equal(t, modelsindex.InstalledStatus, model.Status)
		assert.Equal(t, uint64(1000), model.Size)
	})
}

// fakeSSE records what a download published, in order.
type fakeSSE struct {
	events []render.SSEEvent
	errors []render.SSEErrorData
}

func (f *fakeSSE) Send(event render.SSEEvent)          { f.events = append(f.events, event) }
func (f *fakeSSE) SendError(event render.SSEErrorData) { f.errors = append(f.errors, event) }

func (f *fakeSSE) types() []string {
	types := make([]string, 0, len(f.events))
	for _, e := range f.events {
		types = append(types, e.Type)
	}
	return types
}

// TestDownloadStream covers the translation from a handler's events to SSE, which both
// install routes share. The "done" event is not built here: the route owns it, because
// only the route knows whether a declaration describes the model.
func TestDownloadStream(t *testing.T) {
	t.Run("an info line becomes a message", func(t *testing.T) {
		sse := &fakeSSE{}
		stream := &downloadStream{sse: sse}

		stream.publish(modelsindex.NewInfoMessage("Downloading to: /models/llamacpp", nil))

		require.Equal(t, []string{"message"}, sse.types())
		assert.Equal(t, sseLog{Message: "Downloading to: /models/llamacpp"}, sse.events[0].Data)
		assert.False(t, stream.failed)
		assert.Nil(t, stream.downloaded)
	})

	t.Run("a progress line reports the file name and a percentage", func(t *testing.T) {
		sse := &fakeSSE{}
		stream := &downloadStream{sse: sse}

		stream.publish(modelsindex.NewProgressMessage(modelsindex.Progress{
			Name: "a-model-Q4_0.gguf", Current: 50, Total: 200,
		}))

		require.Equal(t, []string{"progress"}, sse.types())
		assert.Equal(t, sseProgress{
			Name: "a-model-Q4_0.gguf", Current: 50, Total: 200, Progress: 25,
		}, sse.events[0].Data)
	})

	t.Run("a progress line with no total reports no progress", func(t *testing.T) {
		// A handler that has not resolved the size yet sends total 0. Dividing by it would
		// put +Inf or NaN in the stream, which is not valid JSON.
		sse := &fakeSSE{}
		stream := &downloadStream{sse: sse}

		stream.publish(modelsindex.NewProgressMessage(modelsindex.Progress{
			Name: "a-model-Q4_0.gguf", Current: 50, Total: 0,
		}))

		require.Equal(t, []string{"progress"}, sse.types())
		assert.Equal(t, float32(0), sse.events[0].Data.(sseProgress).Progress)
	})

	t.Run("an error line is sent and marks the download failed", func(t *testing.T) {
		// The route reads failed to stop before "done": the 200 is already sent, so the
		// only way to report the failure is the event.
		sse := &fakeSSE{}
		stream := &downloadStream{sse: sse}

		stream.publish(modelsindex.NewErrorMessage("repository does not exist"))

		require.Equal(t, []string{"error"}, sse.types())
		assert.Equal(t, "repository does not exist", sse.events[0].Data)
		assert.True(t, stream.failed)
	})

	t.Run("the handler's own done line is a message, not the route's done", func(t *testing.T) {
		sse := &fakeSSE{}
		stream := &downloadStream{sse: sse}

		stream.publish(modelsindex.NewDoneMessage("download complete"))

		require.Equal(t, []string{"message"}, sse.types())
		assert.Equal(t, sseLog{Message: "download complete"}, sse.events[0].Data)
	})

	t.Run("the model the handler names is kept, and a later line does not clear it", func(t *testing.T) {
		// The id of an undeclared model exists only in this event, so losing it costs the
		// route its answer.
		sse := &fakeSSE{}
		stream := &downloadStream{sse: sse}
		named := &modelsindex.DownloadedModel{ID: "llamacpp:owner/repo/a-model-Q4_0", Size: 1024}

		stream.publish(modelsindex.NewInfoMessage("Downloaded to: /models/llamacpp/owner/repo", named))
		stream.publish(modelsindex.NewInfoMessage("Generated models.ini with 2 model(s)", nil))

		require.NotNil(t, stream.downloaded)
		assert.Equal(t, "llamacpp:owner/repo/a-model-Q4_0", stream.downloaded.ID)
		assert.Equal(t, uint64(1024), stream.downloaded.Size)
	})
}

func TestDownloadStreamSendError(t *testing.T) {
	t.Run("a full models directory carries its own code", func(t *testing.T) {
		// Wrapped, as Download reports it: the client shows a different message for a
		// disk that is full than for a download that broke.
		sse := &fakeSSE{}
		stream := &downloadStream{sse: sse}

		stream.sendError(fmt.Errorf("cannot download: %w", modelsindex.ErrInsufficientStorage))

		require.Len(t, sse.errors, 1)
		assert.Equal(t, render.SSEErrCode("insufficient_storage"), sse.errors[0].Code)
	})

	t.Run("anything else is an internal error carrying the reason", func(t *testing.T) {
		sse := &fakeSSE{}
		stream := &downloadStream{sse: sse}

		stream.sendError(errors.New("handler exited with status 1"))

		require.Len(t, sse.errors, 1)
		assert.Equal(t, render.InternalServiceErr, sse.errors[0].Code)
		assert.Equal(t, "handler exited with status 1", sse.errors[0].Message)
	})
}

// sseRecorder is a ResponseRecorder that render.NewSSEStream accepts: it wants a writer
// it can set a write deadline on, which the recorder alone does not provide.
type sseRecorder struct{ *httptest.ResponseRecorder }

func (sseRecorder) SetWriteDeadline(time.Time) error { return nil }

func testModelsIndex(t *testing.T) *modelsindex.ModelsIndex {
	t.Helper()
	idx, err := modelsindex.Load(platform.GetPlatform(nil), paths.New("testdata"),
		paths.New(t.TempDir()), nil, nil, config.Configuration{})
	require.NoError(t, err)
	return idx
}

// TestHandleInstallModel covers what the install route answers before its stream opens.
// Everything here is decided by the declaration alone, so no container runs and the
// docker client is never touched.
func TestHandleInstallModel(t *testing.T) {
	t.Run("an id the model list does not declare is a 404, not a stream", func(t *testing.T) {
		// The failure has to arrive as a status: once the stream opens the 200 is sent and
		// a client can no longer tell a bad request from a broken download.
		rec := httptest.NewRecorder()
		unknown := modelsindex.EncodeID("llamacpp:no-such-model")
		req := httptest.NewRequest(http.MethodPut, "/v1/models/"+unknown, nil)
		req.SetPathValue("modelID", unknown)

		HandleInstallModel(nil, testModelsIndex(t), platform.GetPlatform(nil))(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.NotContains(t, rec.Header().Get("Content-Type"), "text/event-stream")
		assert.Contains(t, rec.Body.String(), "is declared",
			"the answer says the model list does not declare it")
	})

	t.Run("a declaration that installs the model answers done at once", func(t *testing.T) {
		// Pre-loaded: there is no handler to run, and no progress to report.
		rec := sseRecorder{httptest.NewRecorder()}
		req := httptest.NewRequest(http.MethodPut, "/v1/models/"+modelsindex.EncodeID("a-preloaded-model"), nil)
		req.SetPathValue("modelID", modelsindex.EncodeID("a-preloaded-model"))

		HandleInstallModel(nil, testModelsIndex(t), platform.GetPlatform(nil))(rec, req)

		assert.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")
		body := rec.Body.String()
		assert.Contains(t, body, "event: done")
		assert.Contains(t, body, `"id":"`+modelsindex.EncodeID("a-preloaded-model")+`"`)
		assert.Contains(t, body, `"id_decoded":"a-preloaded-model"`)
		assert.Contains(t, body, `"status":"installed"`)
		assert.NotContains(t, body, "event: progress")
	})

	t.Run("a declared id sent base64url encoded resolves to the same model", func(t *testing.T) {
		rec := sseRecorder{httptest.NewRecorder()}
		req := httptest.NewRequest(http.MethodPut, "/v1/models/"+modelsindex.EncodeID("a-preloaded-model"), nil)
		req.SetPathValue("modelID", modelsindex.EncodeID("a-preloaded-model"))

		HandleInstallModel(nil, testModelsIndex(t), platform.GetPlatform(nil))(rec, req)

		body := rec.Body.String()
		assert.Contains(t, body, "event: done")
		assert.Contains(t, body, `"id_decoded":"a-preloaded-model"`, "the same model, whatever form was asked for")
	})

	t.Run("an id that is not base64url is refused", func(t *testing.T) {
		// The wire form is base64url and nothing else. A client still sending the plain
		// id gets told so whenever that id carries a ":", which every namespaced one does.
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/v1/models/x", nil)
		req.SetPathValue("modelID", "llamacpp:owner/repo")

		HandleInstallModel(nil, testModelsIndex(t), platform.GetPlatform(nil))(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "base64url")
	})

	t.Run("a well-formed id naming no declaration is not found", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/v1/models/x", nil)
		req.SetPathValue("modelID", modelsindex.EncodeID("no-such-model"))

		HandleInstallModel(nil, testModelsIndex(t), platform.GetPlatform(nil))(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// TestHandleDownloadModel covers the body checks. A well-formed url is not tested here:
// it starts the downloader container, which is a hardware test.
func TestHandleDownloadModel(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"an absent body", ""},
		{"a body that is not json", "not json"},
		{"a body naming no url", `{}`},
		{"an empty url", `{"model_url":""}`},
		{"a url of nothing but spaces", `{"model_url":"   "}`},
	} {
		t.Run(tc.name+" is a bad request", func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/models", strings.NewReader(tc.body))

			HandleDownloadModel(nil, testModelsIndex(t), platform.GetPlatform(nil))(rec, req)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.NotContains(t, rec.Header().Get("Content-Type"), "text/event-stream")
		})
	}
}

// encodeModelID is what a client does to put an id in a path: base64url, unpadded.
// TestHandlerModelByID covers the id encoding on the read path. Only a model installed by
// its declaration is used, because that is the one answer no listing container is needed
// for.
func TestHandlerModelByID(t *testing.T) {
	t.Run("an encoded id answers the model, named both ways", func(t *testing.T) {
		segment := modelsindex.EncodeID("a-preloaded-model")
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/models/"+segment, nil)
		req.SetPathValue("modelID", segment)

		HandlerModelByID(testModelsIndex(t))(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		// The answer reports the id encoded, ready to paste back into a path, and plainly
		// beside it for a person to read and for an app.yaml to hold.
		assert.Contains(t, rec.Body.String(), `"id":"`+segment+`"`)
		assert.Contains(t, rec.Body.String(), `"id_decoded":"a-preloaded-model"`)
	})

	t.Run("a plain id is no longer accepted", func(t *testing.T) {
		// Superseded contract: a client sends back the encoded "id" a response gave it.
		// Which failure a leftover plain id gets depends on the id itself - this one is
		// not valid base64url and is refused outright, while one that happens to be
		// decodes to bytes naming no model and gets a 404. Both are failures, which is
		// the point; see TestEncodeDecodeID for the split.
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/models/a-preloaded-model", nil)
		req.SetPathValue("modelID", "a-preloaded-model")

		HandlerModelByID(testModelsIndex(t))(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("an id sent percent-encoded is refused", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/models/llamacpp:owner%2Frepo", nil)
		req.SetPathValue("modelID", "llamacpp:owner/repo")

		HandlerModelByID(testModelsIndex(t))(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "base64url")
	})

	t.Run("an id nothing declares is not found", func(t *testing.T) {
		rec := httptest.NewRecorder()
		unknown := modelsindex.EncodeID("no-such-model")
		req := httptest.NewRequest(http.MethodGet, "/v1/models/"+unknown, nil)
		req.SetPathValue("modelID", unknown)

		HandlerModelByID(testModelsIndex(t))(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}
