// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/appbook"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricks"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

func TestRouter(t *testing.T) {
	tmp := paths.New(t.TempDir())
	cfg := config.New(tmp.Join("apps"), tmp.Join("data"), tmp.Join("assets"))
	plat := platform.Platform{BoardName: "unoq"}
	idProvider := appid.NewAppProvider(cfg, plat)
	var hooks Hooks
	router := NewRouter(idProvider, &bricksindex.BricksIndex{}, bricks.NewService(bricks.NoModelLookup, &bricksindex.BricksIndex{}), cfg, plat, Hooks{
		AppsStatus: func(ctx context.Context) (map[string]appbook.Status, error) {
			return hooks.AppsStatus(ctx)
		},
		BeforeEdit: func(ctx context.Context, req appbook.AppEditRequest, a app.ArduinoApp) error {
			return hooks.BeforeEdit(ctx, req, a)
		},
		BeforeDelete: func(ctx context.Context, a app.ArduinoApp) error {
			return hooks.BeforeDelete(ctx, a)
		},
	})
	serve := func(method, path, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		return rec
	}

	rec := serve("POST", "/v1/apps", `{"name":"app1","icon":"😃"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	appPath := "/v1/apps/" + created.ID
	app1 := tmp.Join("apps", "app1")

	t.Run("the catalog has no status, the details have the hook status", func(t *testing.T) {
		hooks.AppsStatus = func(context.Context) (map[string]appbook.Status, error) {
			return map[string]appbook.Status{app1.String(): appbook.StatusRunning}, nil
		}
		rec := serve("GET", "/v2/apps?filter=apps", "")
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"name":"app1"`)
		require.NotContains(t, rec.Body.String(), `"status"`)

		rec = serve("GET", appPath, "")
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"status":"running"`)
	})

	t.Run("a hook error stops the edit", func(t *testing.T) {
		hooks.BeforeEdit = func(context.Context, appbook.AppEditRequest, app.ArduinoApp) error { return appbook.ErrAppRunning }
		rec := serve("PATCH", appPath, `{"name":"app2"}`)
		require.Equal(t, http.StatusConflict, rec.Code)
		require.True(t, app1.Exist())
	})

	hooks.BeforeEdit = func(context.Context, appbook.AppEditRequest, app.ArduinoApp) error { return nil }
	rec = serve("PATCH", appPath, `{"description":"edited"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"description":"edited"`)

	t.Run("a hook error stops the delete", func(t *testing.T) {
		hooks.BeforeDelete = func(context.Context, app.ArduinoApp) error { return errors.New("busy") }
		rec := serve("DELETE", "/v1/apps/"+created.ID, "")
		require.Equal(t, http.StatusInternalServerError, rec.Code)
		require.True(t, app1.Exist())
	})

	var deleted bool
	hooks.BeforeDelete = func(context.Context, app.ArduinoApp) error { deleted = true; return nil }
	rec = serve("DELETE", "/v1/apps/"+created.ID, "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, deleted)
	require.False(t, app1.Exist())
}

func TestRouterWithoutStatus(t *testing.T) {
	tmp := paths.New(t.TempDir())
	cfg := config.New(tmp.Join("apps"), tmp.Join("data"), tmp.Join("assets"))
	plat := platform.Platform{BoardName: "unoq"}
	idProvider := appid.NewAppProvider(cfg, plat)
	_, err := appbook.CreateApp(appbook.CreateAppRequest{Name: "app1"}, &bricksindex.BricksIndex{}, idProvider, cfg)
	require.NoError(t, err)
	router := NewRouter(idProvider, &bricksindex.BricksIndex{}, bricks.NewService(bricks.NoModelLookup, &bricksindex.BricksIndex{}), cfg, plat, NoHooks)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/apps/"+f.Must(idProvider.ParseID("user:app1")).String(), nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"name":"app1"`)
	require.NotContains(t, rec.Body.String(), `"status"`)
}
