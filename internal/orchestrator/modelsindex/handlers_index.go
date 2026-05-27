// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package modelsindex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/arduino/go-paths-helper"
	"github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

type HandlerActions struct {
	Download string `yaml:"download"`
	Delete   string `yaml:"delete"`
	Check    string `yaml:"check"`
	Info     string `yaml:"info"`
}

func (a HandlerActions) validate(id string) error {
	if a.Download == "" {
		return fmt.Errorf("handler %q: missing required action \"download\"", id)
	}
	if a.Delete == "" {
		return fmt.Errorf("handler %q: missing required action \"delete\"", id)
	}
	if a.Check == "" {
		return fmt.Errorf("handler %q: missing required action \"check\"", id)
	}
	if a.Info == "" {
		return fmt.Errorf("handler %q: missing required action \"info\"", id)
	}
	return nil
}

type ModelHandler struct {
	ID      string
	Image   string
	Actions HandlerActions
}

type HandlersIndex struct {
	handlers map[string]ModelHandler
}

func (h *HandlersIndex) GetHandlerByID(id string) (ModelHandler, bool) {
	handler, ok := h.handlers[id]
	return handler, ok
}

func (h *HandlersIndex) AllHandlers() []ModelHandler {
	handlers := make([]ModelHandler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		handlers = append(handlers, handler)
	}
	return handlers
}

type handlerModelListOutput struct {
	Event  string              `json:"event"`
	Models []handlerModelEntry `json:"models"`
}

type handlerModelEntry struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Handler   string `json:"handler"`
	Platform  string `json:"platform"`
	ModelType string `json:"model_type"`
	Path      string `json:"path"`
	Installed bool   `json:"installed"`
}

// InstalledStatuses runs the list action for every handler and returns a map
// of model ID → installed status.
func (h *HandlersIndex) InstalledStatuses(ctx context.Context, cfg config.Configuration, plat platform.Platform) map[string]bool {
	result := make(map[string]bool)
	for _, handler := range h.handlers {
		entries, err := runListAction(ctx, handler, cfg, plat)
		if err != nil {
			slog.Warn("cannot list models from handler", "handler", handler.ID, "err", err)
			continue
		}
		for _, entry := range entries {
			result[entry.ID] = entry.Installed
		}
	}
	return result
}

func runListAction(ctx context.Context, handler ModelHandler, cfg config.Configuration, plat platform.Platform) ([]handlerModelEntry, error) {
	handlerModelsDir := cfg.CustomModelsDir().Join(handler.ID)

	args := []string{
		"docker", "run", "--rm",
		"-v", fmt.Sprintf("%s:/models", handlerModelsDir),
	}
	if plat.BoardName != "" {
		args = append(args, "-e", fmt.Sprintf("board=%s", plat.BoardName))
	}
	args = append(args, handler.Image, "list_models.sh")

	slog.Debug("running list action", "handler", handler.ID, "image", handler.Image)
	process, err := paths.NewProcess(nil, args...)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	process.RedirectStdoutTo(&buf)
	process.RedirectStderrTo(slog.NewLogLogger(slog.Default().Handler(), slog.LevelDebug).Writer())

	if err := process.RunWithinContext(ctx); err != nil {
		return nil, fmt.Errorf("list action failed: %w", err)
	}

	var output handlerModelListOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		return nil, fmt.Errorf("parsing list output: %w", err)
	}

	return output.Models, nil
}

// rawHandlerEntry wraps HandlerActions with the per-handler image field.
type rawHandlerEntry struct {
	Image          string `yaml:"image"`
	HandlerActions `yaml:",inline"`
}

type rawHandlersList struct {
	Handlers []map[string]rawHandlerEntry `yaml:"handlers"`
}

func loadHandlers(dir *paths.Path, registryBase string) (*HandlersIndex, error) {
	empty := &HandlersIndex{handlers: make(map[string]ModelHandler)}
	if dir == nil {
		return empty, nil
	}

	handlersFile := dir.Join("models-handlers.yaml")
	if handlersFile.NotExist() {
		return empty, nil
	}

	content, err := handlersFile.ReadFile()
	if err != nil {
		return nil, err
	}

	var raw rawHandlersList
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return nil, fmt.Errorf("models-handlers.yaml: %w", err)
	}

	handlers := make(map[string]ModelHandler, len(raw.Handlers))
	for _, handlerMap := range raw.Handlers {
		for id, entry := range handlerMap {
			if id == "" {
				return nil, fmt.Errorf("models-handlers.yaml: handler has empty id")
			}
			if entry.Image == "" {
				return nil, fmt.Errorf("models-handlers.yaml: handler %q missing required field \"image\"", id)
			}
			if err := entry.validate(id); err != nil {
				return nil, fmt.Errorf("models-handlers.yaml: %w", err)
			}
			handlers[id] = ModelHandler{
				ID:      id,
				Image:   fmt.Sprintf("%s%s", registryBase, entry.Image),
				Actions: entry.HandlerActions,
			}
		}
	}

	return &HandlersIndex{handlers: handlers}, nil
}
