// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package boardless serves the daemon's API for a Host with no board: App Lab desktop
// in-process, or the PWA as WebAssembly. It is the board's router with no engine, so
// apps are files, their status is omitted, and what runs them answers 501 "needs_board".
// The docker client is not in its import graph.
package boardless

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/api"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricks"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

// Options is what the Host states. The directories come from the daemon's environment
// variables: ARDUINO_APP_CLI__APPS_DIR, ARDUINO_APP_CLI__DATA_DIR (the catalog in
// assets/<runner version>) and ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR.
type Options struct {
	// Board is the board the apps are made for; empty is an UNO Q.
	Board   string
	Version string
}

// ErrUnknownBoard is returned for an Options.Board that is not a supported board.
var ErrUnknownBoard = errors.New("unknown board")

// NewHandler returns the API for opts, or why it cannot serve it.
func NewHandler(opts Options) (http.Handler, error) {
	if opts.Board == "" {
		opts.Board = platform.BoardUnoQ
	}
	plat, ok := platform.ForBoard(opts.Board)
	if !ok {
		return nil, fmt.Errorf("%w %q: supported are %v", ErrUnknownBoard, opts.Board, platform.SupportedBoards())
	}
	cfg, err := config.NewFromEnv()
	if err != nil {
		return nil, err
	}
	for _, dir := range []*paths.Path{cfg.AppsDir(), cfg.DataDir()} {
		if err := dir.MkdirAll(); err != nil {
			return nil, fmt.Errorf("creating %s: %w", dir, err)
		}
	}

	bricksIndex, err := bricksindex.Load(plat, cfg.AssetDir())
	if err != nil {
		return nil, fmt.Errorf("loading the bricks catalog: %w", err)
	}
	modelsIndex, err := modelsindex.Load(plat, cfg.AssetDir(), cfg.ModelsDir(), cfg.CustomModelsDir(), nil, cfg)
	if err != nil {
		return nil, fmt.Errorf("loading the models catalog: %w", err)
	}
	servicesIndex, err := servicesindex.Load(plat, cfg.AssetDir().Join("services"))
	if err != nil {
		return nil, fmt.Errorf("loading the services catalog: %w", err)
	}

	return api.NewHTTPRouter(
		nil, // no engine: no board
		opts.Version,
		nil, // no updater: a Host updates itself
		nil, // no provisioner
		modelsIndex,
		bricksIndex,
		servicesIndex,
		bricks.NewService(modelsIndex, bricksIndex),
		appid.NewAppProvider(cfg, plat),
		plat,
		cfg,
		nil, // no build events
		nil, // no websocket origins: the Host is the only caller
	), nil
}
