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

	"github.com/arduino/go-paths-helper"
	"github.com/docker/docker/client"

	"github.com/arduino/arduino-app-cli/internal/dockerhandler"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
)

func runHandlerAction(ctx context.Context, cli client.APIClient, model modelsindex.AIModel, image string, action []string, downloadPath *paths.Path, envVars map[string]string, publish func(ModelInstallEvent)) error {
	env := make([]string, 0, len(envVars))
	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	var stdout io.Writer
	if publish != nil {
		stdout = &handlerOutputParser{publish: publish}
	} else {
		stdout = slog.NewLogLogger(slog.Default().Handler(), slog.LevelDebug).Writer()
	}

	slog.Debug("running handler action", "model", model.ID, "action", action, "image", image)
	return dockerhandler.Run(ctx, cli, dockerhandler.RunOptions{
		Image:  image,
		Cmd:    action,
		Binds:  []string{fmt.Sprintf("%s:/models", downloadPath)},
		Env:    env,
		Stdout: stdout,
		Stderr: slog.NewLogLogger(slog.Default().Handler(), slog.LevelDebug).Writer(),
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
