// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package boardless_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/pkg/boardless"
	"github.com/arduino/go-paths-helper"
)

// setEnv points the handler at a fresh workspace, with the testdata catalog where the
// board keeps it, and returns the apps directory.
func setEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	appsDir := filepath.Join(dir, "ArduinoApps")
	t.Setenv("ARDUINO_APP_CLI__APPS_DIR", appsDir)
	t.Setenv("ARDUINO_APP_CLI__DATA_DIR", filepath.Join(dir, "data"))
	t.Setenv("ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR", filepath.Join(dir, "custom-models"))
	cfg, err := config.NewFromEnv()
	require.NoError(t, err)
	require.NoError(t, paths.New("testdata", "assets").CopyDirTo(cfg.AssetDir()))
	return appsDir
}

func newServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	appsDir := setEnv(t)
	h, err := boardless.NewHandler(boardless.Options{Version: "test"})
	require.NoError(t, err)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, appsDir
}

func do(t *testing.T, srv *httptest.Server, method, path string, body any) (int, []byte) {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, srv.URL+path, r)
	require.NoError(t, err)
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, out
}

func createApp(t *testing.T, srv *httptest.Server, name string) string {
	t.Helper()
	code, body := do(t, srv, http.MethodPost, "/v1/apps", map[string]any{"name": name, "icon": "🧪"})
	require.Equal(t, http.StatusCreated, code, string(body))
	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(body, &created))
	require.NotEmpty(t, created.ID)
	return url.PathEscape(created.ID)
}

func TestNewHandlerErrors(t *testing.T) {
	t.Run("unknown board", func(t *testing.T) {
		setEnv(t)
		_, err := boardless.NewHandler(boardless.Options{Board: "mega"})
		require.ErrorIs(t, err, boardless.ErrUnknownBoard)
	})
	t.Run("missing catalog is an error, not a panic", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("ARDUINO_APP_CLI__APPS_DIR", filepath.Join(dir, "a"))
		t.Setenv("ARDUINO_APP_CLI__DATA_DIR", filepath.Join(dir, "d"))
		t.Setenv("ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR", filepath.Join(dir, "m"))
		_, err := boardless.NewHandler(boardless.Options{})
		require.Error(t, err)
	})
	t.Run("ventunoq", func(t *testing.T) {
		setEnv(t)
		_, err := boardless.NewHandler(boardless.Options{Board: "ventunoq"})
		require.NoError(t, err)
	})
}

func TestCatalog(t *testing.T) {
	srv, _ := newServer(t)

	code, body := do(t, srv, http.MethodGet, "/v1/version", nil)
	require.Equal(t, http.StatusOK, code)
	assert.JSONEq(t, `{"version":"test"}`, string(body))

	code, body = do(t, srv, http.MethodGet, "/v1/bricks", nil)
	require.Equal(t, http.StatusOK, code, string(body))
	assert.Contains(t, string(body), "arduino:dbstorage_sqlstore")

	code, body = do(t, srv, http.MethodGet, "/v1/models", nil)
	require.Equal(t, http.StatusOK, code, string(body))
	assert.Contains(t, string(body), "ei-model-123-1")

	code, body = do(t, srv, http.MethodGet, "/v1/config", nil)
	require.Equal(t, http.StatusOK, code, string(body))

}

func TestApps(t *testing.T) {
	srv, appsDir := newServer(t)
	id := createApp(t, srv, "first")

	t.Run("the app is in the Local workspace", func(t *testing.T) {
		_, err := os.Stat(filepath.Join(appsDir, "first", "app.yaml"))
		require.NoError(t, err)
	})

	t.Run("list and details omit the status", func(t *testing.T) {
		code, body := do(t, srv, http.MethodGet, "/v1/apps", nil)
		require.Equal(t, http.StatusOK, code, string(body))
		var list struct {
			Apps []map[string]any `json:"apps"`
		}
		require.NoError(t, json.Unmarshal(body, &list))
		require.Len(t, list.Apps, 1)
		assert.NotContains(t, list.Apps[0], "status")

		code, body = do(t, srv, http.MethodGet, "/v1/apps/"+id, nil)
		require.Equal(t, http.StatusOK, code, string(body))
		var details map[string]any
		require.NoError(t, json.Unmarshal(body, &details))
		assert.Equal(t, "first", details["name"])
		assert.NotContains(t, details, "status")
	})

	t.Run("edit", func(t *testing.T) {
		code, body := do(t, srv, http.MethodPatch, "/v1/apps/"+id, map[string]any{"description": "edited"})
		require.Equal(t, http.StatusOK, code, string(body))
		_, body = do(t, srv, http.MethodGet, "/v1/apps/"+id, nil)
		assert.Contains(t, string(body), `"edited"`)
	})

	t.Run("bricks", func(t *testing.T) {
		code, body := do(t, srv, http.MethodPut, "/v1/apps/"+id+"/bricks/arduino:dbstorage_sqlstore", map[string]any{})
		require.Equal(t, http.StatusOK, code, string(body))
		code, body = do(t, srv, http.MethodGet, "/v1/apps/"+id+"/bricks", nil)
		require.Equal(t, http.StatusOK, code, string(body))
		assert.Contains(t, string(body), "arduino:dbstorage_sqlstore")
	})

	t.Run("clone", func(t *testing.T) {
		code, body := do(t, srv, http.MethodPost, "/v1/apps/"+id+"/clone", map[string]any{"name": "second"})
		require.Equal(t, http.StatusCreated, code, string(body))
	})

	t.Run("export, delete, import", func(t *testing.T) {
		code, zip := do(t, srv, http.MethodGet, "/v1/apps/"+id+"/export", nil)
		require.Equal(t, http.StatusOK, code, string(zip))

		code, body := do(t, srv, http.MethodDelete, "/v1/apps/"+id, nil)
		require.Equal(t, http.StatusOK, code, string(body))
		_, err := os.Stat(filepath.Join(appsDir, "first"))
		require.ErrorIs(t, err, os.ErrNotExist)

		var form bytes.Buffer
		w := multipart.NewWriter(&form)
		part, err := w.CreateFormFile("file", "first.zip")
		require.NoError(t, err)
		_, err = part.Write(zip)
		require.NoError(t, err)
		require.NoError(t, w.Close())
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/v1/apps/import", &form)
		require.NoError(t, err)
		req.Header.Set("Content-Type", w.FormDataContentType())
		resp, err := srv.Client().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		out, _ := io.ReadAll(resp.Body)
		require.Equal(t, http.StatusCreated, resp.StatusCode, string(out))
		_, err = os.Stat(filepath.Join(appsDir, "first", "app.yaml"))
		require.NoError(t, err)
	})
}

func TestProperties(t *testing.T) {
	srv, _ := newServer(t)
	code, body := do(t, srv, http.MethodPut, "/v1/properties/theme", map[string]any{"value": "dark"})
	require.True(t, code == http.StatusOK || code == http.StatusNoContent, "%d %s", code, body)
	code, body = do(t, srv, http.MethodGet, "/v1/properties/theme", nil)
	require.Equal(t, http.StatusOK, code, string(body))
	assert.Contains(t, string(body), "dark")
}

func TestNeedsBoardRoutes(t *testing.T) {
	srv, _ := newServer(t)
	id := createApp(t, srv, "app")
	for _, r := range []struct{ method, path string }{
		{http.MethodGet, "/v1/apps/events"},
		{http.MethodGet, "/v1/apps/" + id + "/logs"},
		{http.MethodPost, "/v1/apps/" + id + "/start"},
		{http.MethodPost, "/v1/apps/" + id + "/stop"},
		{http.MethodPost, "/v1/apps/" + id + "/build"},
		{http.MethodGet, "/v1/apps/build/events"},
		{http.MethodPut, "/v1/apps/install"},
		{http.MethodPut, "/v1/apps/" + id + "/prepare"},
		{http.MethodPost, "/v1/models"},
		{http.MethodPut, "/v1/models/ei-model-123-1"},
		{http.MethodPut, "/v1/models/ei/projects/1"},
		{http.MethodDelete, "/v1/models/ei-model-123-1"},
		{http.MethodGet, "/v1/system/update/check"},
		{http.MethodGet, "/v1/system/update/events"},
		{http.MethodPut, "/v1/system/update/apply"},
	} {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			code, body := do(t, srv, r.method, r.path, nil)
			require.Equal(t, http.StatusNotImplemented, code, string(body))
			assert.Contains(t, string(body), `"code":"needs_board"`)
		})
	}
}
