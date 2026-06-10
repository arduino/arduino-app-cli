// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	paths "github.com/arduino/go-paths-helper"
	"github.com/docker/docker/client"

	"github.com/arduino/arduino-app-cli/internal/dockerhandler"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
)

func runHandlerAction(ctx context.Context, cli client.APIClient, model modelsindex.AIModel, image string, action []string, downloadPath *paths.Path, envVars map[string]string, publish func(ModelInstallEvent)) error {
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
		Image:  image,
		Cmd:    action,
		Binds:  []string{fmt.Sprintf("%s:/models", downloadPath)},
		Env:    env,
		Stdout: &handlerOutputParser{publish: publish},
		Stderr: io.Discard, // TODO: consider parsing stderr as well, or at least logging it
	})
}

type handlerOutputParser struct {
	publish func(ModelInstallEvent)
	buf     []byte
}

func (p *handlerOutputParser) Write(b []byte) (int, error) {
	p.buf = append(p.buf, b...)
	for {
		idx := bytes.IndexByte(p.buf, '\n')
		if idx == -1 {
			break
		}
		line := bytes.TrimSpace(p.buf[:idx])
		p.buf = p.buf[idx+1:]
		if len(line) == 0 {
			continue
		}
		p.parseLine(line)
	}
	return len(b), nil
}

func (p *handlerOutputParser) parseLine(line []byte) {
	var raw struct {
		Event       string   `json:"event"`
		Description string   `json:"description"`
		Current     int64    `json:"current"`
		Total       int64    `json:"total"`
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
	default:
		eventType = ModelInstallEventInfo
	}
	p.publish(ModelInstallEvent{
		Type:        eventType,
		Description: raw.Description,
		Current:     raw.Current,
		Total:       raw.Total,
		Unit:        raw.Unit,
		Percentage:  raw.Percentage,
		Artifacts:   raw.Artifacts,
	})
}
