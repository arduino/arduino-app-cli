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
	"os"
	"strings"

	"github.com/arduino/go-paths-helper"
	"github.com/docker/docker/client"
	"github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/dockerhandler"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

type HandlerActions struct {
	Download []string
	Delete   []string
	Check    []string
	Info     []string
}

func (a HandlerActions) validate(id string) error {
	if len(a.Download) == 0 {
		return fmt.Errorf("handler %q: missing required action \"download\"", id)
	}
	if len(a.Delete) == 0 {
		return fmt.Errorf("handler %q: missing required action \"delete\"", id)
	}
	if len(a.Check) == 0 {
		return fmt.Errorf("handler %q: missing required action \"check\"", id)
	}
	return nil
}

type ModelHandler struct {
	ID      string
	Image   string
	Volumes []string
	Actions HandlerActions
}

// ResolveVolumes resolves template variables in each volume spec against vars.
// It returns the Docker bind-mount strings (binds) and an "KEY=value" slice
// (envAdditions) for every template variable that was substituted, so the
// container can also reference the paths through environment variables.
func ResolveVolumes(vols []string, vars map[string]string) (binds, envAdditions []string) {
	seen := make(map[string]bool)
	for _, vol := range vols {
		expanded := os.Expand(vol, func(key string) string {
			varName, defaultVal := key, ""
			if idx := strings.Index(key, ":-"); idx != -1 {
				varName, defaultVal = key[:idx], key[idx+2:]
			}
			val, ok := vars[varName]
			if !ok {
				return defaultVal
			}
			if !seen[varName] {
				envAdditions = append(envAdditions, fmt.Sprintf("%s=%s", varName, val))
				seen[varName] = true
			}
			return val
		})
		binds = append(binds, expanded)
	}
	return
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

// GetModelStatus runs the list action for every handler and returns a map
// of model ID → installed status.
func (h *HandlersIndex) GetModelStatus(ctx context.Context, cli client.APIClient, cfg config.Configuration, plat platform.Platform) map[string]bool {
	result := make(map[string]bool)
	for _, handler := range h.handlers {
		entries, err := runListAction(ctx, cli, handler, cfg, plat)
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

func runListAction(ctx context.Context, cli client.APIClient, handler ModelHandler, cfg config.Configuration, plat platform.Platform) ([]handlerModelEntry, error) {
	handlerModelsDir := cfg.CustomModelsDir()

	var env []string
	if plat.BoardName != "" {
		env = append(env, fmt.Sprintf("board=%s", plat.BoardName))
	}

	slog.Debug("running list action", "handler", handler.ID, "image", handler.Image)

	binds, volumeEnv := ResolveVolumes(handler.Volumes, map[string]string{
		"ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR": handlerModelsDir.String(),
	})
	env = append(env, volumeEnv...)

	var buf bytes.Buffer
	err := dockerhandler.Run(ctx, cli, dockerhandler.RunOptions{
		Image:  handler.Image,
		Cmd:    []string{"/app/list_models.sh"},
		Binds:  binds,
		Env:    env,
		Stdout: &buf,
	})
	if err != nil {
		return nil, fmt.Errorf("list action: %w", err)
	}

	var output handlerModelListOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		return nil, fmt.Errorf("parsing list output: %w", err)
	}

	return output.Models, nil
}

type rawActionEntry struct {
	Command []string `yaml:"command"`
}

type rawHandlerEntry struct {
	Description string                      `yaml:"description"`
	Image       string                      `yaml:"image"`
	Volumes     []string                    `yaml:"volumes"`
	Actions     []map[string]rawActionEntry `yaml:"actions"`
}

type rawHandlersList struct {
	Handlers []map[string]rawHandlerEntry `yaml:"handlers"`
}

// resolveImage replaces a ${VAR:-default} prefix in the image string with registryBase.
func resolveImage(raw, registryBase string) string {
	if start := strings.Index(raw, "${"); start != -1 {
		if end := strings.Index(raw[start:], "}"); end != -1 {
			return registryBase + raw[start+end+1:]
		}
	}
	return registryBase + raw
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
			var actions HandlerActions
			for _, actionMap := range entry.Actions {
				for name, actionEntry := range actionMap {
					switch name {
					case "download":
						actions.Download = actionEntry.Command
					case "delete":
						actions.Delete = actionEntry.Command
					case "check":
						actions.Check = actionEntry.Command
					case "info":
						actions.Info = actionEntry.Command
					}
				}
			}
			if err := actions.validate(id); err != nil {
				return nil, fmt.Errorf("models-handlers.yaml: %w", err)
			}
			if len(entry.Volumes) == 0 {
				return nil, fmt.Errorf("models-handlers.yaml: handler %q missing required field \"volumes\"", id)
			}
			handlers[id] = ModelHandler{
				ID:      id,
				Image:   resolveImage(entry.Image, registryBase),
				Volumes: entry.Volumes,
				Actions: actions,
			}
		}
	}

	return &HandlersIndex{handlers: handlers}, nil
}
