// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validHash = "0000000000000000000000000000000000000000000000000000000000000000"

const validIndex = `{
  "bricks": [
    {"id": "acme:camera", "namespace": "acme", "name": "Camera Brick", "version": "1.0.0",
     "url": "https://example.com/camera-1.0.0.zip", "hash": "` + validHash + `", "size": 1}
  ]
}`

func TestParseRemoteIndexValidation(t *testing.T) {
	// release returns a valid release entry, with the given fields replaced.
	release := func(replace map[string]any) map[string]any {
		r := map[string]any{
			"id": "acme:camera", "namespace": "acme", "name": "Camera Brick", "version": "1.0.0",
			"url": "https://example.com/camera.zip", "hash": validHash, "size": 1,
		}
		maps.Copy(r, replace)
		for k, v := range r {
			if v == nil {
				delete(r, k)
			}
		}
		return r
	}
	parse := func(releases ...map[string]any) error {
		data, err := json.Marshal(map[string]any{"bricks": releases})
		require.NoError(t, err)
		_, err = parseRemoteIndex(data)
		return err
	}

	require.NoError(t, parse(release(nil), release(map[string]any{"version": "1.1.0"})))
	require.NoError(t, parse(release(map[string]any{"id": "acme_2:_my_camera2", "namespace": "acme_2"})))

	tests := []struct {
		name    string
		release map[string]any
		wantErr string
	}{
		{"id without namespace", release(map[string]any{"id": "camera"}), "invalid brick id"},
		{"uppercase in id", release(map[string]any{"id": "acme:Camera"}), "invalid brick id"},
		{"dash in id", release(map[string]any{"id": "acme:my-camera"}), "invalid brick id"},
		{"leading digit in id", release(map[string]any{"id": "acme:2camera"}), "invalid brick id"},
		{"dot in namespace", release(map[string]any{"id": "acme.io:camera", "namespace": "acme.io"}), "invalid brick id"},
		{"namespace mismatch", release(map[string]any{"namespace": "other"}), "does not match"},
		{"missing version", release(map[string]any{"version": nil}), "missing version"},
		{"empty version", release(map[string]any{"version": ""}), "missing version"},
		{"invalid version", release(map[string]any{"version": "v1"}), "decoding bricks index"},
		{"missing name", release(map[string]any{"name": nil}), "missing name"},
		{"missing url", release(map[string]any{"url": nil}), "invalid url"},
		{"relative url", release(map[string]any{"url": "camera.zip"}), "invalid url"},
		{"unsupported url scheme", release(map[string]any{"url": "file:///camera.zip"}), "invalid url"},
		{"missing hash", release(map[string]any{"hash": nil}), "invalid SHA-256 hash"},
		{"hash with algorithm prefix", release(map[string]any{"hash": "SHA-256:" + validHash}), "invalid SHA-256 hash"},
		{"short hash", release(map[string]any{"hash": "abcd"}), "invalid SHA-256 hash"},
		{"missing size", release(map[string]any{"size": nil}), "invalid size"},
		{"negative size", release(map[string]any{"size": -1}), "invalid size"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorContains(t, parse(tt.release), tt.wantErr)
		})
	}

	t.Run("duplicate release", func(t *testing.T) {
		err := parse(release(map[string]any{"version": "1.2"}), release(map[string]any{"version": "1.2.0"}))
		require.ErrorContains(t, err, "duplicate release acme:camera@1.2.0")
	})
}

// serveFile serves body at the returned URL. With chunked, the size of the body is not declared.
func serveFile(t *testing.T, status int, body []byte, chunked bool) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !chunked {
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		}
		w.WriteHeader(status)
		if chunked {
			w.(http.Flusher).Flush()
		}
		if r.Method != http.MethodHead {
			_, _ = w.Write(body)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/file"
}

func mustParseURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	require.NoError(t, err)
	return u
}

func TestUpdate(t *testing.T) {
	dir := paths.New(t.TempDir()).Join("bricks-index")
	body := validIndex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-agent", r.Header.Get("User-Agent"))
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	u := mustParseURL(t, srv.URL)
	config := DownloadConfig("test-agent")

	index, err := UpdateRemoteIndex(t.Context(), config, u, dir)
	require.NoError(t, err)
	require.Equal(t, "acme:camera", index.Bricks[0].ID)
	saved, err := dir.Join(RemoteIndexFileName).ReadFile()
	require.NoError(t, err)
	require.Equal(t, validIndex, string(saved), "the index must be saved as downloaded")

	// A saved index is replaced.
	body = strings.ReplaceAll(validIndex, "acme:camera", "acme:audio")
	index, err = UpdateRemoteIndex(t.Context(), config, u, dir)
	require.NoError(t, err)
	require.Equal(t, "acme:audio", index.Bricks[0].ID)
}

func TestRemoteIndexIsRecent(t *testing.T) {
	dir := paths.New(t.TempDir())
	require.False(t, RemoteIndexIsRecent(dir, time.Hour), "a missing index is not recent")

	indexPath := dir.Join(RemoteIndexFileName)
	require.NoError(t, indexPath.WriteFile([]byte(validIndex)))
	require.True(t, RemoteIndexIsRecent(dir, time.Hour))

	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(indexPath.String(), old, old))
	require.False(t, RemoteIndexIsRecent(dir, time.Hour))
}
func TestUpdateKeepsPreviousIndexOnFailure(t *testing.T) {
	tests := []struct {
		name    string
		url     func(t *testing.T) string
		wantErr string
	}{
		{"server error", func(t *testing.T) string {
			return serveFile(t, http.StatusInternalServerError, nil, false)
		}, "500"},
		{"malformed json", func(t *testing.T) string {
			return serveFile(t, http.StatusOK, []byte("{"), false)
		}, "decoding bricks index"},
		{"invalid index", func(t *testing.T) string {
			return serveFile(t, http.StatusOK, []byte(`{"bricks": [{"id": "camera", "namespace": "acme"}]}`), false)
		}, "invalid brick id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := paths.New(t.TempDir())
			require.NoError(t, dir.Join(RemoteIndexFileName).WriteFile([]byte(validIndex)))

			_, err := UpdateRemoteIndex(t.Context(), DownloadConfig("test"), mustParseURL(t, tt.url(t)), dir)
			require.ErrorContains(t, err, tt.wantErr)

			saved, err := dir.Join(RemoteIndexFileName).ReadFile()
			require.NoError(t, err)
			require.Equal(t, validIndex, string(saved))
			// No temporary files are left behind.
			entries, err := dir.ReadDir()
			require.NoError(t, err)
			require.Len(t, entries, 1)
		})
	}
}

func TestUpdateInactivityTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	config := DownloadConfig("test")
	config.InactivityTimeout = 100 * time.Millisecond

	_, err := UpdateRemoteIndex(t.Context(), config, mustParseURL(t, srv.URL), paths.New(t.TempDir()))
	require.Error(t, err)
}
