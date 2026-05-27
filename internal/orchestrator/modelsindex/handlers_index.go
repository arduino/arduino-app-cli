// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package modelsindex

import (
	"fmt"

	"github.com/arduino/go-paths-helper"
	"github.com/goccy/go-yaml"
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
			if err := entry.HandlerActions.validate(id); err != nil {
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
