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
	"log/slog"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
)

func runHandlerAction(ctx context.Context, model modelsindex.AIModel, image string, action string, downloadPath *paths.Path, publish func(ModelInstallEvent)) error {
	args := []string{
		"docker", "run", "--rm",
		"-v", fmt.Sprintf("%s:/models", downloadPath),
		image,
		action,
		model.ID,
	}

	slog.Debug("running handler action", "model", model.ID, "action", action, "image", image)
	process, err := paths.NewProcess(nil, args...)
	if err != nil {
		return err
	}
	process.RedirectStderrTo(slog.NewLogLogger(slog.Default().Handler(), slog.LevelDebug).Writer())
	if publish != nil {
		process.RedirectStdoutTo(&handlerOutputParser{publish: publish})
	} else {
		process.RedirectStdoutTo(slog.NewLogLogger(slog.Default().Handler(), slog.LevelDebug).Writer())
	}
	return process.RunWithinContext(ctx)
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
		Event       string `json:"event"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(line, &raw); err != nil {
		slog.Debug("non-JSON stdout from handler", "line", string(line))
		return
	}
	eventType := ModelInstallEventInfo
	if raw.Event == "error" {
		eventType = ModelInstallEventError
	}
	p.publish(ModelInstallEvent{Type: eventType, Description: raw.Description})
}
