// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

// With a nil engine, what runs apps answers 501 needs_board.
func TestNilEngineNeedsBoard(t *testing.T) {
	t.Setenv("ARDUINO_APP_CLI__APPS_DIR", t.TempDir())
	t.Setenv("ARDUINO_APP_CLI__DATA_DIR", t.TempDir())
	cfg, err := config.NewFromEnv()
	require.NoError(t, err)
	plat := platform.Platform{BoardName: "unoq"}
	router := NewHTTPRouter(nil, "test", nil, nil, nil, nil, nil, nil, appid.NewAppProvider(cfg, plat), plat, cfg, nil, nil)

	for _, route := range []struct{ method, path string }{
		{"GET", "/v1/apps/events"},
		{"GET", "/v1/apps/x/logs"},
		{"POST", "/v1/apps/x/start"},
		{"POST", "/v1/apps/x/stop"},
		{"POST", "/v1/apps/x/build"},
		{"PUT", "/v1/apps/install"},
		{"PUT", "/v1/apps/x/prepare"},
		{"PUT", "/v1/models/m"},
		{"POST", "/v1/models"},
		{"DELETE", "/v1/models/m"},
		{"PUT", "/v1/models/ei/projects/1"},
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(route.method, route.path, nil))
		require.Equal(t, http.StatusNotImplemented, rec.Code, route)
		require.Contains(t, rec.Body.String(), "needs_board", route)
	}
}
