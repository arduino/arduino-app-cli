// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

type buildResponsePart struct {
	header map[string][]string
	body   []byte
}

// flushRecorder counts the flushes: one for the headers, then one for each heartbeat.
type flushRecorder struct {
	*httptest.ResponseRecorder
	flushes atomic.Int32
}

func (r *flushRecorder) Flush() { r.flushes.Add(1) }

func readBuildResponse(t *testing.T, recorder *httptest.ResponseRecorder) []buildResponsePart {
	t.Helper()

	require.Equal(t, http.StatusOK, recorder.Code)
	mediaType, params, err := mime.ParseMediaType(recorder.Header().Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/mixed", mediaType)

	var parts []buildResponsePart
	reader := multipart.NewReader(recorder.Body, params["boundary"])
	for {
		part, err := reader.NextRawPart()
		if err == io.EOF {
			return parts
		}
		require.NoError(t, err)
		body, err := io.ReadAll(part)
		require.NoError(t, err)
		parts = append(parts, buildResponsePart{header: part.Header, body: body})
	}
}

func TestBuildResponse(t *testing.T) {
	const heartbeat = 10 * time.Millisecond

	t.Run("the archive is the last part, after the heartbeats", func(t *testing.T) {
		archive := paths.New(t.TempDir(), "app.ard")
		require.NoError(t, archive.WriteFile([]byte("archive content")))

		recorder := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
		response := startBuildResponse(recorder, heartbeat)
		require.Eventually(t, func() bool { return recorder.flushes.Load() >= 3 }, 5*time.Second, time.Millisecond,
			"no heartbeat was sent while the build was running")
		require.NoError(t, response.SendArchive(archive, "app.ard"))

		parts := readBuildResponse(t, recorder.ResponseRecorder)
		require.GreaterOrEqual(t, len(parts), 3)
		for _, part := range parts[:len(parts)-1] {
			assert.Equal(t, []string{"text/plain"}, part.header["Content-Type"])
			assert.Empty(t, part.body)
		}
		last := parts[len(parts)-1]
		assert.Equal(t, []string{"application/gzip"}, last.header["Content-Type"])
		assert.Equal(t, []string{`attachment; filename="app.ard"`}, last.header["Content-Disposition"])
		assert.Equal(t, "archive content", string(last.body))
	})

	t.Run("a failed build ends with the error in place of the archive", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		response := startBuildResponse(recorder, heartbeat)
		response.SendError(models.ErrorResponse{Code: "INTERNAL_SERVER_ERROR", Details: "the build failed"})

		parts := readBuildResponse(t, recorder)
		require.NotEmpty(t, parts)
		last := parts[len(parts)-1]
		assert.Equal(t, []string{"application/json"}, last.header["Content-Type"])
		assert.JSONEq(t, `{"event":"error","code":"INTERNAL_SERVER_ERROR","details":"the build failed"}`, string(last.body))
	})

	t.Run("an archive that cannot be read ends with an error", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		response := startBuildResponse(recorder, heartbeat)
		require.Error(t, response.SendArchive(paths.New(t.TempDir(), "missing.ard"), "missing.ard"))

		parts := readBuildResponse(t, recorder)
		require.NotEmpty(t, parts)
		last := parts[len(parts)-1]
		assert.Equal(t, []string{"application/json"}, last.header["Content-Type"])
		var got models.ErrorResponse
		require.NoError(t, json.Unmarshal(last.body, &got))
		assert.Equal(t, "INTERNAL_SERVER_ERROR", got.Code)
	})

	t.Run("an archive that fails while it is sent is left without the closing boundary", func(t *testing.T) {
		// A directory opens, and fails at the first read.
		recorder := httptest.NewRecorder()
		response := startBuildResponse(recorder, heartbeat)
		require.Error(t, response.SendArchive(paths.New(t.TempDir()), "app.ard"))

		_, params, err := mime.ParseMediaType(recorder.Header().Get("Content-Type"))
		require.NoError(t, err)
		body := recorder.Body.String()
		assert.Contains(t, body, "Content-Type: application/gzip")
		assert.NotContains(t, body, "--"+params["boundary"]+"--")
	})

	t.Run("the progress and the messages are sent as JSON parts, in order", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		response := startBuildResponse(recorder, time.Hour)
		response.SendProgress(&orchestrator.Progress{Name: "sketch", Progress: 80}, "building sketch")
		response.SendProgress(nil, "compiling")
		response.SendError(models.ErrorResponse{Details: "the build failed"})
		// Dropped: the last part is already sent.
		response.SendProgress(nil, "too late")

		parts := readBuildResponse(t, recorder)
		require.Len(t, parts, 4)
		for _, part := range parts {
			assert.Equal(t, []string{"application/json"}, part.header["Content-Type"])
		}
		assert.JSONEq(t, `{"event":"progress","name":"sketch","progress":80}`, string(parts[0].body))
		assert.JSONEq(t, `{"event":"message","message":"building sketch"}`, string(parts[1].body))
		assert.JSONEq(t, `{"event":"message","message":"compiling"}`, string(parts[2].body))
		assert.JSONEq(t, `{"event":"error","details":"the build failed"}`, string(parts[3].body))
	})

	t.Run("the progress and the heartbeat are written one part at a time", func(t *testing.T) {
		archive := paths.New(t.TempDir(), "app.ard")
		require.NoError(t, archive.WriteFile([]byte("archive content")))

		recorder := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
		response := startBuildResponse(recorder, time.Millisecond)
		var senders sync.WaitGroup
		for range 4 {
			senders.Go(func() {
				for range 50 {
					response.SendProgress(&orchestrator.Progress{Name: "sketch", Progress: 80}, "compiling")
				}
			})
		}
		senders.Wait()
		require.NoError(t, response.SendArchive(archive, "app.ard"))

		// Parts written together would not parse.
		parts := readBuildResponse(t, recorder.ResponseRecorder)
		var events int
		for _, part := range parts[:len(parts)-1] {
			if part.header["Content-Type"][0] == "application/json" {
				require.True(t, json.Valid(part.body), "%s", part.body)
				events++
			}
		}
		assert.Equal(t, 400, events)
		assert.Equal(t, "archive content", string(parts[len(parts)-1].body))
	})

	t.Run("a second Close does not block", func(t *testing.T) {
		response := startBuildResponse(httptest.NewRecorder(), heartbeat)
		response.SendError(models.ErrorResponse{Details: "the build failed"})

		closed := make(chan struct{})
		go func() {
			defer close(closed)
			response.Close()
			response.Close()
		}()
		select {
		case <-closed:
		case <-time.After(5 * time.Second):
			t.Fatal("Close blocked")
		}
	})
}
