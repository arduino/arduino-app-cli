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

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-app-cli/internal/render"
)

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
// install routes share. The "done" event is the route's, not this translation's.
func TestDownloadStream(t *testing.T) {
	t.Run("an info line becomes a message", func(t *testing.T) {
		sse := &fakeSSE{}
		stream := &downloadStream{sse: sse}

		stream.publish(modelsindex.NewInfoMessage("Downloading to: /models/llamacpp", nil))

		require.Equal(t, []string{"message"}, sse.types())
		assert.Equal(t, sseLog{Message: "Downloading to: /models/llamacpp"}, sse.events[0].Data)
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

	t.Run("an error line is sent as an event", func(t *testing.T) {
		// The 200 is already sent, so the only way to report a failure is the event.
		sse := &fakeSSE{}
		stream := &downloadStream{sse: sse}

		stream.publish(modelsindex.NewErrorMessage("repository does not exist"))

		assert.Empty(t, sse.events)
		require.Len(t, sse.errors, 1)
		assert.Equal(t, render.SSEErrorData{
			Code: render.InternalServiceErr, Message: "repository does not exist",
		}, sse.errors[0])
	})

	t.Run("the handler's own done line is a message, not the route's done", func(t *testing.T) {
		sse := &fakeSSE{}
		stream := &downloadStream{sse: sse}

		stream.publish(modelsindex.NewDoneMessage("download complete"))

		require.Equal(t, []string{"message"}, sse.types())
		assert.Equal(t, sseLog{Message: "download complete"}, sse.events[0].Data)
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
// The declaration decides all of it, so no container runs.
func TestHandleInstallModel(t *testing.T) {
	t.Run("an id the model list does not declare is a 404, not a stream", func(t *testing.T) {
		// The failure has to arrive as a status: once the stream opens the 200 is sent and
		// a client can no longer tell a bad request from a broken download.
		rec := httptest.NewRecorder()
		unknown := models.EncodeModelID("llamacpp:no-such-model")
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
		req := httptest.NewRequest(http.MethodPut, "/v1/models/"+models.EncodeModelID("a-preloaded-model"), nil)
		req.SetPathValue("modelID", models.EncodeModelID("a-preloaded-model"))

		HandleInstallModel(nil, testModelsIndex(t), platform.GetPlatform(nil))(rec, req)

		assert.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")
		body := rec.Body.String()
		assert.Contains(t, body, "event: done")
		assert.Contains(t, body, `"id":"`+models.EncodeModelID("a-preloaded-model")+`"`)
		assert.Contains(t, body, `"id_decoded":"a-preloaded-model"`)
		assert.Contains(t, body, `"status":"installed"`)
		assert.NotContains(t, body, "event: progress")
	})

	t.Run("a declared id sent base64url encoded resolves to the same model", func(t *testing.T) {
		rec := sseRecorder{httptest.NewRecorder()}
		req := httptest.NewRequest(http.MethodPut, "/v1/models/"+models.EncodeModelID("a-preloaded-model"), nil)
		req.SetPathValue("modelID", models.EncodeModelID("a-preloaded-model"))

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
		req.SetPathValue("modelID", models.EncodeModelID("no-such-model"))

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

// TestHandlerModelByID covers the id encoding on the read path, with a model installed by
// its declaration: the one answer that needs no listing container.
func TestHandlerModelByID(t *testing.T) {
	t.Run("an encoded id answers the model, named both ways", func(t *testing.T) {
		segment := models.EncodeModelID("a-preloaded-model")
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
		// A leftover plain id fails either way: this one is not valid base64url, while one
		// that happens to be decodes to bytes naming no model and gets a 404.
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
		unknown := models.EncodeModelID("no-such-model")
		req := httptest.NewRequest(http.MethodGet, "/v1/models/"+unknown, nil)
		req.SetPathValue("modelID", unknown)

		HandlerModelByID(testModelsIndex(t))(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}
