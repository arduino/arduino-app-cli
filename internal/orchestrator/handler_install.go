// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	paths "github.com/arduino/go-paths-helper"
	"github.com/docker/docker/client"

	"github.com/arduino/arduino-app-cli/internal/dockerhandler"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
)

func runHandlerAction(ctx context.Context, cli client.APIClient, model modelsindex.AIModel, image string, action []string, volumes []string, downloadPath *paths.Path, envVars map[string]string, publish func(ModelInstallEvent)) error {
	modelRepo := envVars["models_repository"]
	if i := strings.Index(modelRepo, "/"); i >= 0 {
		modelRepo = modelRepo[i+1:]
	}
	bindPath := downloadPath
	if modelRepo != "" {
		bindPath = downloadPath.Join(modelRepo)
	}
	if err := bindPath.MkdirAll(); err != nil {
		return fmt.Errorf("cannot create model directory %q: %w", bindPath, err)
	}
	if err := os.Chmod(bindPath.String(), 0777); err != nil { //nolint:gosec
		slog.Warn("cannot set permissions on model directory", "path", bindPath, "err", err)
	}
	binds := modelsindex.ResolveVolumes(volumes, map[string]string{
		"ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR": bindPath.String(),
	})

	env := make([]string, 0, len(envVars))
	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	if publish == nil {
		publish = func(e ModelInstallEvent) {
			slog.Debug("handler event", "model", model.ID, "event", e.Type, "description", e.Description)
		}
	}

	slog.Debug("running handler action", "model", model.ID, "action", action, "image", image)
	return dockerhandler.Run(ctx, cli, dockerhandler.RunOptions{
		Image: image,
		Cmd:   action,
		Binds: binds,
		Env:   env,
		LineCallback: func(line string) {
			slog.Debug("handler output", "model", model.ID, "line", line)
			parseHandlerLine([]byte(line), publish)
		},
		Stderr: io.Discard,
	})
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
