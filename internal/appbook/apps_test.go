// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package appbook

import (
	"context"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

var (
	unoQPlatform   = platform.Platform{BoardName: "unoq"}
	unkownPlatform = platform.Platform{}
)

// Apps are files: created, listed, shown, renamed and deleted.
func TestAppCRUD(t *testing.T) {
	tmpDir := paths.New(t.TempDir())
	t.Setenv("ARDUINO_APP_CLI__APPS_DIR", tmpDir.Join("apps").String())
	t.Setenv("ARDUINO_APP_CLI__DATA_DIR", tmpDir.Join("data").String())
	cfg, err := config.NewFromEnv()
	require.NoError(t, err)
	idProvider := appid.NewAppProvider(cfg, unoQPlatform)
	bricks := &bricksindex.BricksIndex{}
	noBoard := func(context.Context) (map[string]Status, error) { return nil, nil }

	created, err := CreateApp(CreateAppRequest{Name: "app1", Icon: "😃"}, bricks, idProvider, cfg)
	require.NoError(t, err)

	list, err := ListApps(ListAppRequest{ShowApps: true}, idProvider, bricks, cfg, unoQPlatform)
	require.NoError(t, err)
	require.Len(t, list.Apps, 1)
	require.Equal(t, created.ID, list.Apps[0].ID)

	a, err := app.Load(created.ID.ToPath())
	require.NoError(t, err)
	details, err := AppDetails(t.Context(), a, noBoard, bricks, idProvider, cfg)
	require.NoError(t, err)
	require.Equal(t, "app1", details.Name)

	name := "app2"
	require.NoError(t, EditApp(AppEditRequest{Name: &name}, &a, cfg))
	require.Equal(t, "app2", a.Name)

	require.NoError(t, DeleteApp(a))
	list, err = ListApps(ListAppRequest{ShowApps: true}, idProvider, bricks, cfg, unoQPlatform)
	require.NoError(t, err)
	require.Empty(t, list.Apps)
}
