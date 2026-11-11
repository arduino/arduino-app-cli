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
	"testing"

	"github.com/arduino/go-paths-helper"
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

// indexServer serves body with the given etag, answering 304 when the client already has it.
func indexServer(t *testing.T, status int, body, etag string) *url.URL {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if etag != "" && r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if etag != "" {
			w.Header().Set("ETag", etag)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL + "/brick-index.json")
	require.NoError(t, err)
	return u
}

func TestUpdate(t *testing.T) {
	dir := paths.New(t.TempDir()).Join("bricks-index")
	u := indexServer(t, http.StatusOK, validIndex, `"v1"`)

	res, err := UpdateRemoteIndex(t.Context(), http.DefaultClient, u, dir, "test")
	require.NoError(t, err)
	require.True(t, res.Updated)
	require.Len(t, res.Index.Bricks, 1)

	saved, err := dir.Join(RemoteIndexFileName).ReadFile()
	require.NoError(t, err)
	require.Equal(t, validIndex, string(saved), "the index must be saved as downloaded")

	t.Run("an unchanged index is not downloaded again", func(t *testing.T) {
		res, err := UpdateRemoteIndex(t.Context(), http.DefaultClient, u, dir, "test")
		require.NoError(t, err)
		require.False(t, res.Updated)
		require.Equal(t, "acme:camera", res.Index.Bricks[0].ID)
	})

	t.Run("a missing local index is downloaded even if the etag matches", func(t *testing.T) {
		require.NoError(t, dir.Join(RemoteIndexFileName).Remove())
		res, err := UpdateRemoteIndex(t.Context(), http.DefaultClient, u, dir, "test")
		require.NoError(t, err)
		require.True(t, res.Updated)
	})
}

func TestUpdateKeepsPreviousIndexOnFailure(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		errMsg string
	}{
		{"server error", http.StatusInternalServerError, "", "unexpected status"},
		{"malformed json", http.StatusOK, "{", "decoding bricks index"},
		{"invalid index", http.StatusOK, `{"bricks": [{"id": "camera", "namespace": "acme"}]}`, "invalid brick id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := paths.New(t.TempDir())
			require.NoError(t, dir.Join(RemoteIndexFileName).WriteFile([]byte(validIndex)))

			u := indexServer(t, tt.status, tt.body, "")
			_, err := UpdateRemoteIndex(t.Context(), http.DefaultClient, u, dir, "test")
			require.ErrorContains(t, err, tt.errMsg)

			saved, err := dir.Join(RemoteIndexFileName).ReadFile()
			require.NoError(t, err)
			require.Equal(t, validIndex, string(saved))
		})
	}
}
