// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

// With no engine, as without a board, apps are files: listed, shown, renamed and
// deleted, with no status.
func TestNoEngine(t *testing.T) {
	cfg := setTestOrchestratorConfig(t)
	idProvider := appid.NewAppProvider(cfg, unoQPlatform)
	id := createApp(t, "app1", false, idProvider, cfg)

	list, err := ListApps(t.Context(), nil, ListAppRequest{ShowApps: true}, idProvider, &bricksindex.BricksIndex{}, cfg, unoQPlatform)
	require.NoError(t, err)
	require.Len(t, list.Apps, 1)
	require.Equal(t, Status(""), list.Apps[0].Status)

	a, err := app.Load(id.ToPath())
	require.NoError(t, err)
	details, err := AppDetails(t.Context(), nil, a, &bricksindex.BricksIndex{}, idProvider, cfg)
	require.NoError(t, err)
	require.Equal(t, Status(""), details.Status)

	name := "app2"
	require.NoError(t, EditApp(t.Context(), nil, AppEditRequest{Name: &name}, &a, cfg))

	require.NoError(t, DeleteApp(t.Context(), nil, unoQPlatform, a, cfg))
	list, err = ListApps(t.Context(), nil, ListAppRequest{ShowApps: true}, idProvider, &bricksindex.BricksIndex{}, cfg, unoQPlatform)
	require.NoError(t, err)
	require.Empty(t, list.Apps)
}
