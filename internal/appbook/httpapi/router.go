// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package httpapi serves the app CRUD of appbook over HTTP.
package httpapi

import (
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricks"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

func NewRouter(
	idProvider *appid.Provider,
	bricksIndex *bricksindex.BricksIndex,
	brickService *bricks.Service,
	cfg config.Configuration,
	platform platform.Platform,
	hooks Hooks,
) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /v1/config", HandleConfig(cfg))
	mux.Handle("GET /v1/examples", HandleExamples(cfg, bricksIndex, idProvider))
	mux.Handle("GET /v1/bricks", HandleBrickList(brickService))
	mux.Handle("GET /v1/bricks/{brickID}", HandleBrickDetails(brickService, idProvider, cfg, platform))

	mux.Handle("POST /v1/apps", HandleAppCreate(bricksIndex, idProvider, cfg))
	mux.Handle("GET /v1/apps/{appID}", HandleAppDetails(bricksIndex, idProvider, cfg, hooks.AppsStatus))
	mux.Handle("PATCH /v1/apps/{appID}", HandleAppDetailsEdits(bricksIndex, idProvider, cfg, hooks))
	mux.Handle("DELETE /v1/apps/{appID}", HandleAppDelete(idProvider, hooks.BeforeDelete))
	mux.Handle("POST /v1/apps/{appID}/clone", HandleAppClone(idProvider, cfg))
	mux.Handle("GET /v1/apps/{appID}/export", HandleAppExport(idProvider, bricksIndex))
	mux.Handle("POST /v1/apps/import", HandleAppImport(cfg, idProvider))
	mux.Handle("GET /v1/apps/{appID}/bricks", HandleAppBrickInstancesList(brickService, idProvider))
	mux.Handle("GET /v1/apps/{appID}/bricks/{brickID}", HandleAppBrickInstanceDetails(brickService, idProvider))
	mux.Handle("PUT /v1/apps/{appID}/bricks/{brickID}", HandleBrickCreate(brickService, idProvider))
	mux.Handle("PATCH /v1/apps/{appID}/bricks/{brickID}", HandleBrickUpdates(brickService, idProvider))
	mux.Handle("DELETE /v1/apps/{appID}/bricks/{brickID}", HandleBrickDelete(brickService, idProvider))

	mux.Handle("GET /v2/apps", HandleAppList(idProvider, bricksIndex, cfg, platform))
	return mux
}
