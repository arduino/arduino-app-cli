// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package remotebricksindex

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
)

const validIndex = `{
  "bricks": [
    {
      "id": "acme:camera",
      "name": "Camera Brick",
      "namespace": "acme",
      "latest": "1.0.0",
      "releases": [{"version": "1.0.0", "url": "https://example.com/camera-1.0.0.zip", "hash": "0000000000000000000000000000000000000000000000000000000000000000", "size": 1}]
    }
  ]
}`

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

	res, err := Update(t.Context(), http.DefaultClient, u, dir, "test")
	require.NoError(t, err)
	require.True(t, res.Updated)
	require.Len(t, res.Index.Bricks, 1)

	saved, err := dir.Join(IndexFileName).ReadFile()
	require.NoError(t, err)
	require.Equal(t, validIndex, string(saved), "the index must be saved as downloaded")

	t.Run("an unchanged index is not downloaded again", func(t *testing.T) {
		res, err := Update(t.Context(), http.DefaultClient, u, dir, "test")
		require.NoError(t, err)
		require.False(t, res.Updated)
		require.Equal(t, "acme:camera", res.Index.Bricks[0].ID)
	})

	t.Run("a missing local index is downloaded even if the etag matches", func(t *testing.T) {
		require.NoError(t, dir.Join(IndexFileName).Remove())
		res, err := Update(t.Context(), http.DefaultClient, u, dir, "test")
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
		{"namespace mismatch", http.StatusOK, `{"bricks": [{"id": "acme:camera", "namespace": "other"}]}`, "does not match"},
		{"invalid id", http.StatusOK, `{"bricks": [{"id": "camera", "namespace": "acme"}]}`, "invalid brick id"},
		{"duplicate id", http.StatusOK, `{"bricks": [{"id": "acme:a", "namespace": "acme"}, {"id": "acme:a", "namespace": "acme"}]}`, "duplicate brick id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := paths.New(t.TempDir())
			require.NoError(t, dir.Join(IndexFileName).WriteFile([]byte(validIndex)))

			u := indexServer(t, tt.status, tt.body, "")
			_, err := Update(t.Context(), http.DefaultClient, u, dir, "test")
			require.ErrorContains(t, err, tt.errMsg)

			saved, err := dir.Join(IndexFileName).ReadFile()
			require.NoError(t, err)
			require.Equal(t, validIndex, string(saved))
		})
	}
}
