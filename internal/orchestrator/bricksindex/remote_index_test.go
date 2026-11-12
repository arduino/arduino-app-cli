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
    {"id": "acme:camera", "namespace": "acme", "author": "Acme", "name": "Camera Brick",
     "releases": [{"version": "1.0.0", "url": "https://example.com/camera-1.0.0.zip", "hash": "` + validHash + `", "size": 1}]}
  ]
}`

// withFields returns a copy of m with the given fields replaced. A nil value removes the field.
func withFields(m, replace map[string]any) map[string]any {
	res := maps.Clone(m)
	maps.Copy(res, replace)
	for k, v := range res {
		if v == nil {
			delete(res, k)
		}
	}
	return res
}

func TestParseRemoteIndexValidation(t *testing.T) {
	// release returns a valid release, with the given fields replaced.
	release := func(replace map[string]any) map[string]any {
		return withFields(map[string]any{"version": "1.0.0", "url": "https://example.com/camera.zip", "hash": validHash, "size": 1}, replace)
	}
	// brick returns a valid brick with a valid release, with the given fields replaced.
	brick := func(replace map[string]any) map[string]any {
		return withFields(map[string]any{
			"id": "acme:camera", "namespace": "acme", "author": "Acme", "name": "Camera Brick",
			"releases": []map[string]any{release(nil)},
		}, replace)
	}
	parse := func(bricks ...map[string]any) error {
		data, err := json.Marshal(map[string]any{"bricks": bricks})
		require.NoError(t, err)
		_, err = parseRemoteIndex(data)
		return err
	}
	withRelease := func(replace map[string]any) map[string]any {
		return brick(map[string]any{"releases": []map[string]any{release(replace)}})
	}

	require.NoError(t, parse(brick(nil), brick(map[string]any{"id": "acme:audio"})))
	require.NoError(t, parse(brick(map[string]any{"id": "acme_2:_my_camera2", "namespace": "acme_2"})))
	require.NoError(t, parse(brick(map[string]any{"releases": []map[string]any{release(nil), release(map[string]any{"version": "1.1.0"})}})))

	tests := []struct {
		name    string
		brick   map[string]any
		wantErr string
	}{
		{"id without namespace", brick(map[string]any{"id": "camera"}), "invalid brick id"},
		{"uppercase in id", brick(map[string]any{"id": "acme:Camera"}), "invalid brick id"},
		{"dash in id", brick(map[string]any{"id": "acme:my-camera"}), "invalid brick id"},
		{"leading digit in id", brick(map[string]any{"id": "acme:2camera"}), "invalid brick id"},
		{"dot in namespace", brick(map[string]any{"id": "acme.io:camera", "namespace": "acme.io"}), "invalid brick id"},
		{"namespace mismatch", brick(map[string]any{"namespace": "other"}), "does not match"},
		{"missing name", brick(map[string]any{"name": nil}), "missing name"},
		{"missing author", brick(map[string]any{"author": nil}), "missing author"},
		{"missing releases", brick(map[string]any{"releases": nil}), "no releases"},
		{"empty releases", brick(map[string]any{"releases": []map[string]any{}}), "no releases"},
		{"missing version", withRelease(map[string]any{"version": nil}), "missing version"},
		{"empty version", withRelease(map[string]any{"version": ""}), "missing version"},
		{"invalid version", withRelease(map[string]any{"version": "v1"}), "decoding bricks index"},
		{"missing url", withRelease(map[string]any{"url": nil}), "invalid url"},
		{"relative url", withRelease(map[string]any{"url": "camera.zip"}), "invalid url"},
		{"unsupported url scheme", withRelease(map[string]any{"url": "file:///camera.zip"}), "invalid url"},
		{"missing hash", withRelease(map[string]any{"hash": nil}), "invalid SHA-256 hash"},
		{"hash with algorithm prefix", withRelease(map[string]any{"hash": "SHA-256:" + validHash}), "invalid SHA-256 hash"},
		{"short hash", withRelease(map[string]any{"hash": "abcd"}), "invalid SHA-256 hash"},
		{"missing size", withRelease(map[string]any{"size": nil}), "invalid size"},
		{"negative size", withRelease(map[string]any{"size": -1}), "invalid size"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorContains(t, parse(tt.brick), tt.wantErr)
		})
	}

	t.Run("duplicate brick", func(t *testing.T) {
		require.ErrorContains(t, parse(brick(nil), brick(nil)), `duplicate brick "acme:camera"`)
	})
	t.Run("duplicate release", func(t *testing.T) {
		err := parse(brick(map[string]any{"releases": []map[string]any{
			release(map[string]any{"version": "1.2"}), release(map[string]any{"version": "1.2.0"}),
		}}))
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
