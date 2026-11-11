// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package appbook serves the app CRUD over HTTP, on the directories the caller chooses.
package appbook

import (
	"fmt"
	"net/http"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/appbook/httpapi"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricks"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

type Config struct {
	// AppsDir holds the apps the user edits.
	AppsDir string
	// DataDir holds the examples, the installed releases, the default app and, in assets, the bricks catalog.
	DataDir string
}

// New serves the app CRUD on the directories of cfg.
func New(cfg Config) (http.Handler, error) {
	dataDir := paths.New(cfg.DataDir)
	c := config.New(paths.New(cfg.AppsDir), dataDir, dataDir.Join("assets"))
	plat := platform.Platform{}
	bricksIndex, err := bricksindex.Load(plat, c.AssetDir())
	if err != nil {
		return nil, fmt.Errorf("failed to load the bricks catalog: %w", err)
	}
	return httpapi.NewRouter(appid.NewAppProvider(c, plat), bricksIndex, bricks.NewService(bricks.NoModelLookup, bricksIndex), c, plat, httpapi.NoHooks), nil
}
