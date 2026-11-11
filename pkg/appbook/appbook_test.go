// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package appbook

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The CRUD over HTTP, as a caller of the package sees it.
func TestNew(t *testing.T) {
	tmp := t.TempDir()
	assets := filepath.Join(tmp, "data", "assets")
	require.NoError(t, os.MkdirAll(assets, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(assets, "bricks-list.yaml"), []byte("bricks: []\n"), 0o600))
	router, err := New(Config{AppsDir: filepath.Join(tmp, "apps"), DataDir: filepath.Join(tmp, "data")})
	require.NoError(t, err)
	serve := func(method, path, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		return rec
	}

	rec := serve("POST", "/v1/apps", `{"name":"app1"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var created struct{ ID string }
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	rec = serve("GET", "/v2/apps?filter=apps", "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"name":"app1"`)

	rec = serve("PATCH", "/v1/apps/"+created.ID, `{"name":"app2"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var renamed struct{ ID string }
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &renamed))
	require.DirExists(t, filepath.Join(tmp, "apps", "app2"))

	rec = serve("GET", "/v1/apps/"+renamed.ID, "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"name":"app2"`)

	rec = serve("DELETE", "/v1/apps/"+renamed.ID, "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoDirExists(t, filepath.Join(tmp, "apps", "app2"))
}
