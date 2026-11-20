// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/dockerhelper"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-app-cli/internal/render"
)

const testBoard = "test-board"

// brickIndexJSON returns an index with acme:camera 1.0.0, for every board, and 1.1.0, for
// another board only, both downloaded from archiveURL.
func brickIndexJSON(t *testing.T, archiveURL, hash string, size int) []byte {
	t.Helper()
	release := func(version string, boards []string) map[string]any {
		return map[string]any{"version": version, "url": archiveURL, "hash": hash, "size": size, "supported_boards": boards}
	}
	data, err := json.Marshal(map[string]any{"bricks": []map[string]any{{
		"id": "acme:camera", "namespace": "acme", "author": "Acme", "name": "Camera",
		"releases": []map[string]any{release("1.0.0", nil), release("1.1.0", []string{"other-board"})},
	}}})
	require.NoError(t, err)
	return data
}

// brickTestConfig returns a configuration with its data in a temporary folder. The index is
// saved there when not nil, and downloaded from indexURL when not empty.
func brickTestConfig(t *testing.T, index []byte, indexURL string) config.Configuration {
	t.Helper()
	tmp := paths.New(t.TempDir())
	t.Setenv("ARDUINO_APP_CLI__DATA_DIR", tmp.Join("data").String())
	t.Setenv("ARDUINO_APP_CLI__APPS_DIR", tmp.Join("apps").String())
	t.Setenv("ARDUINO_APP_CLI__BRICKS_INDEX_URL", indexURL)
	cfg, err := config.NewFromEnv()
	require.NoError(t, err)
	if index != nil {
		require.NoError(t, cfg.BricksIndexDir().MkdirAll())
		require.NoError(t, cfg.BricksIndexDir().Join(bricksindex.RemoteIndexFileName).WriteFile(index))
	}
	return cfg
}

func installBrick(t *testing.T, cfg config.Configuration, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, target, nil)
	req.SetPathValue("brickID", strings.TrimPrefix(strings.SplitN(target, "?", 2)[0], "/v1/bricks/"))
	HandleInstallBrick(nil, cfg, nil, platform.Platform{BoardName: testBoard}, bricksindex.DownloadConfig("test"))(sseRecorder{rec}, req)
	return rec
}

type sseTestEvent struct {
	Type string
	Data string
}

// sseEvents parses the events of body, without the error and close events every stream ends with.
func sseEvents(t *testing.T, body string) []sseTestEvent {
	t.Helper()
	var events []sseTestEvent
	for block := range strings.SplitSeq(strings.TrimSpace(body), "\n\n") {
		var e sseTestEvent
		for line := range strings.SplitSeq(block, "\n") {
			if v, ok := strings.CutPrefix(line, "event: "); ok {
				e.Type = v
			} else if v, ok := strings.CutPrefix(line, "data: "); ok {
				e.Data = v
			}
		}
		events = append(events, e)
	}
	require.GreaterOrEqual(t, len(events), 2)
	tail := events[len(events)-2:]
	require.Equal(t, "error", tail[0].Type)
	require.Contains(t, tail[0].Data, "SERVER_CLOSED")
	require.Equal(t, "close", tail[1].Type)
	return events[:len(events)-2]
}

func eventTypes(events []sseTestEvent) []string {
	types := make([]string, 0, len(events))
	for _, e := range events {
		types = append(types, e.Type)
	}
	return types
}

// TestHandleInstallBrickBeforeStream covers what the route answers with a status, before the stream opens.
func TestHandleInstallBrickBeforeStream(t *testing.T) {
	index := brickIndexJSON(t, "https://example.com/camera.zip", strings.Repeat("0", 64), 1)
	tests := []struct {
		name       string
		target     string
		wantStatus int
		wantBody   string
	}{
		{"invalid id", "/v1/bricks/acme:Camera", http.StatusBadRequest, "invalid brick id"},
		{"id without namespace", "/v1/bricks/camera", http.StatusBadRequest, "invalid brick id"},
		{"empty version", "/v1/bricks/acme:camera?version=", http.StatusBadRequest, "missing version"},
		{"invalid version", "/v1/bricks/acme:camera?version=v1", http.StatusBadRequest, "invalid version"},
		{"brick not in the index", "/v1/bricks/acme:audio", http.StatusNotFound, "brick not found in the bricks index"},
		{"version not in the index", "/v1/bricks/acme:camera?version=2.0.0", http.StatusNotFound, "brick version not found in the bricks index"},
		{"version not supporting the board", "/v1/bricks/acme:camera?version=1.1.0", http.StatusBadRequest, "not supported by this board"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := installBrick(t, brickTestConfig(t, index, ""), tt.target)
			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.NotContains(t, rec.Header().Get("Content-Type"), "text/event-stream")
			assert.Contains(t, rec.Body.String(), tt.wantBody)
		})
	}

	t.Run("no index and no way to download it", func(t *testing.T) {
		rec := installBrick(t, brickTestConfig(t, nil, ""), "/v1/bricks/acme:camera")
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Contains(t, rec.Body.String(), "bricks index not available")
	})
}

func TestHandleInstallBrickAlreadyInstalled(t *testing.T) {
	index := brickIndexJSON(t, "https://example.com/camera.zip", strings.Repeat("0", 64), 1)

	t.Run("the latest release supporting the board is done at once", func(t *testing.T) {
		cfg := brickTestConfig(t, index, "")
		require.NoError(t, cfg.BricksDir().Join("acme", "camera", "1.0.0").MkdirAll())

		rec := installBrick(t, cfg, "/v1/bricks/acme:camera")

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")
		events := sseEvents(t, rec.Body.String())
		require.Equal(t, []string{"done"}, eventTypes(events))
		assert.JSONEq(t, `{"id":"acme:camera","version":"1.0.0","already_installed":true}`, events[0].Data)
	})

	t.Run("a stale index that cannot be updated is used with a warning", func(t *testing.T) {
		cfg := brickTestConfig(t, index, serveStatus(t, http.StatusInternalServerError))
		indexPath := cfg.BricksIndexDir().Join(bricksindex.RemoteIndexFileName)
		old := time.Now().Add(-2 * bricksindex.RemoteIndexMaxAge)
		require.NoError(t, os.Chtimes(indexPath.String(), old, old))
		require.NoError(t, cfg.BricksDir().Join("acme", "camera", "1.0.0").MkdirAll())

		rec := installBrick(t, cfg, "/v1/bricks/acme:camera?version=1.0.0")

		events := sseEvents(t, rec.Body.String())
		require.Equal(t, []string{"message", "done"}, eventTypes(events))
		assert.Contains(t, events[0].Data, "cannot update the bricks index, using the saved one")
	})
}

func TestHandleInstallBrickFailure(t *testing.T) {
	// An archive without the src folder fails after the download, before anything uses docker.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create("README.md")
	require.NoError(t, err)
	_, err = f.Write([]byte("no sources"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	archive := buf.Bytes()
	hash := sha256.Sum256(archive)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(archive)))
		_, _ = w.Write(archive)
	}))
	t.Cleanup(srv.Close)
	cfg := brickTestConfig(t, brickIndexJSON(t, srv.URL+"/camera.zip", hex.EncodeToString(hash[:]), len(archive)), "")

	rec := installBrick(t, cfg, "/v1/bricks/acme:camera")

	require.Equal(t, http.StatusOK, rec.Code)
	events := sseEvents(t, rec.Body.String())
	require.NotEmpty(t, events)
	assert.Equal(t, "progress", events[0].Type)
	assert.Contains(t, events[0].Data, `"name":"acme:camera@1.0.0"`)
	last := events[len(events)-1]
	assert.Equal(t, "error", last.Type)
	assert.Contains(t, last.Data, string(render.InternalServiceErr))
	assert.Contains(t, last.Data, "src")
	assert.NotContains(t, eventTypes(events), "done")
	assert.NoDirExists(t, cfg.BricksDir().Join("acme", "camera", "1.0.0").String())
}

func TestBrickInstallStream(t *testing.T) {
	t.Run("the download is reported once per percent", func(t *testing.T) {
		sse := &fakeSSE{}
		progress := (&brickInstallStream{sse: sse}).downloadProgress("acme:camera@1.0.0")

		// 150 and 1050 stay at the percent of the previous chunk.
		for _, downloaded := range []int64{100, 150, 1000, 1050, 10000} {
			progress(downloaded, 10000)
		}

		require.Equal(t, []string{"progress", "progress", "progress"}, sse.types())
		assert.Equal(t, sseProgress{Name: "acme:camera@1.0.0", Current: 100, Total: 10000, Progress: 1}, sse.events[0].Data)
		assert.Equal(t, sseProgress{Name: "acme:camera@1.0.0", Current: 1000, Total: 10000, Progress: 10}, sse.events[1].Data)
		assert.Equal(t, sseProgress{Name: "acme:camera@1.0.0", Current: 10000, Total: 10000, Progress: 100}, sse.events[2].Data)
	})

	t.Run("a full disk carries its own code", func(t *testing.T) {
		for _, err := range []error{
			fmt.Errorf("downloading brick acme:camera@1.0.0: %w", bricksindex.ErrInsufficientStorage),
			fmt.Errorf("installing brick acme:camera@1.0.0: %w", dockerhelper.ErrOutOfSpace),
		} {
			sse := &fakeSSE{}
			(&brickInstallStream{sse: sse}).sendError(err)
			require.Len(t, sse.errors, 1)
			assert.Equal(t, render.SSEErrorData{Code: "insufficient_storage", Message: err.Error()}, sse.errors[0])
		}
	})

	t.Run("anything else is an internal error carrying the reason", func(t *testing.T) {
		sse := &fakeSSE{}
		(&brickInstallStream{sse: sse}).sendError(fmt.Errorf("downloading brick acme:camera@1.0.0: SHA-256 mismatch"))
		require.Len(t, sse.errors, 1)
		assert.Equal(t, render.InternalServiceErr, sse.errors[0].Code)
		assert.Contains(t, sse.errors[0].Message, "SHA-256 mismatch")
	})
}

func serveStatus(t *testing.T, status int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}
