// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	paths "github.com/arduino/go-paths-helper"
	"github.com/docker/docker/client"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
)

func runHandlerAction(ctx context.Context, cli client.APIClient, model modelsindex.AIModel, image string, action []string, volumes []string, downloadPath *paths.Path, envVars map[string]string, publish func(ModelInstallEvent)) error {

	bindPath := downloadPath
	if envVars["models_repository"] != "" {
		bindPath = downloadPath.Join(envVars["models_repository"])
	}
	if err := bindPath.MkdirAll(); err != nil {
		return fmt.Errorf("cannot create model directory %q: %w", bindPath, err)
	}
	if err := os.Chmod(bindPath.String(), 0777); err != nil { //nolint:gosec
		slog.Warn("cannot set permissions on model directory", "path", bindPath, "err", err)
	}

	if publish == nil {
		publish = func(e ModelInstallEvent) {
			slog.Debug("handler event", "model", model.ID, "event", e.Type, "description", e.Description)
		}
	}

	slog.Debug("running handler action", "model", model.ID, "action", action, "image", image)
	return nil
}

func parseHandlerLine(line []byte, publish func(ModelInstallEvent)) {
	var raw struct {
		Event       string   `json:"event"`
		Description string   `json:"description"`
		Current     int64    `json:"current"`
		Total       int64    `json:"total"`
		SizeMB      float64  `json:"size_mb"`
		Unit        string   `json:"unit"`
		Percentage  string   `json:"percentage"`
		Artifacts   []string `json:"artifacts"`
	}
	if err := json.Unmarshal(line, &raw); err != nil {
		slog.Debug("non-JSON stdout from handler", "line", string(line))
		return
	}
	var eventType ModelInstallEventType
	switch raw.Event {
	case "start":
		eventType = ModelInstallEventStart
	case "update":
		eventType = ModelInstallEventUpdate
	case "complete":
		eventType = ModelInstallEventComplete
	case "error":
		eventType = ModelInstallEventError
	case "stat":
		eventType = ModelInstallEventInfo
		if raw.SizeMB > 0 {
			raw.Total = int64(raw.SizeMB * 1024 * 1024)
		}
	default:
		eventType = ModelInstallEventInfo
	}
	publish(ModelInstallEvent{
		Type:        eventType,
		Description: raw.Description,
		Current:     raw.Current,
		Total:       raw.Total,
		Unit:        raw.Unit,
		Percentage:  raw.Percentage,
		Artifacts:   raw.Artifacts,
	})
}
