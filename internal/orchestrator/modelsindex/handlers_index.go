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
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/arduino/go-paths-helper"
	"github.com/docker/docker/client"

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

type ListingConfig struct {
	Image   string
	Volumes []string
	Command []string
}

type HandlersIndex struct {
	handlers map[string]ModelHandler
	listing  *ListingConfig
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
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Handler     string   `json:"handler"`
	Platform    string   `json:"platform"`
	ModelType   string   `json:"model_type"`
	Path        string   `json:"path"`
	Installed   bool     `json:"installed"`
	ModelSizeMB *float64 `json:"model_size_mb"` // from yaml metadata
	DiskSizeMB  *float64 `json:"disk_size_mb"`  // actual on-disk size, only when installed
}

func (h *HandlersIndex) getModelInfo(ctx context.Context, cli client.APIClient, models []AIModel, modelsDir *paths.Path, plat platform.Platform) {
	if h == nil || h.listing == nil {
		return
	}
	entries, err := runListAction(ctx, cli, h.listing, modelsDir, plat)
	if err != nil {
		slog.Warn("cannot list models", "err", err)
		return
	}
	idx := make(map[string]int, len(models))
	for i, m := range models {
		idx[m.ID] = i
	}
	for _, entry := range entries {
		i, ok := idx[entry.ID]
		if !ok || models[i].IsInternal {
			continue
		}
		models[i].Installed = entry.Installed
		if entry.Installed && entry.DiskSizeMB != nil && *entry.DiskSizeMB > 0 {
			models[i].Size = uint64(*entry.DiskSizeMB * 1024 * 1024)
		} else if entry.ModelSizeMB != nil && *entry.ModelSizeMB > 0 {
			models[i].Size = uint64(*entry.ModelSizeMB * 1024 * 1024)
		}
	}
}

func runInfoAction(ctx context.Context, cli client.APIClient, handler ModelHandler, model AIModel, modelsDir *paths.Path, plat platform.Platform) (uint64, error) {
	var env []string
	if plat.BoardName != "" {
		env = append(env, fmt.Sprintf("board=%s", plat.BoardName))
	}
	var envVars map[string]string
	if model.Deployment != nil {
		envVars = model.Deployment.VariablesForPlatform(plat.BoardName)
	}
	binds, volumeEnv := ResolveVolumes(handler.Volumes, map[string]string{
		"ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR": modelsDir.String(),
	})
	env = append(env, volumeEnv...)
	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	var size uint64
	err := dockerhandler.Run(ctx, cli, dockerhandler.RunOptions{
		Image: handler.Image,
		Cmd:   handler.Actions.Info,
		Binds: binds,
		Env:   env,
		LineCallback: func(line string) {
			var out struct {
				Event  string  `json:"event"`
				SizeMB float64 `json:"size_mb"`
			}
			if jsonErr := json.Unmarshal([]byte(line), &out); jsonErr == nil && out.Event == "stat" && out.SizeMB > 0 {
				size = uint64(out.SizeMB * 1024 * 1024)
			}
		},
	})
	if err != nil {
		return 0, fmt.Errorf("info action: %w", err)
	}
	return size, nil
}

func runListAction(ctx context.Context, cli client.APIClient, listing *ListingConfig, modelsDir *paths.Path, plat platform.Platform) ([]handlerModelEntry, error) {
	var env []string
	if plat.BoardName != "" {
		env = append(env, fmt.Sprintf("BOARD_NAME=%s", plat.BoardName))
	}

	slog.Debug("running list action", "image", listing.Image)

	binds, volumeEnv := ResolveVolumes(listing.Volumes, map[string]string{
		"ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR": modelsDir.String(),
	})
	env = append(env, volumeEnv...)

	var buf bytes.Buffer
	err := dockerhandler.Run(ctx, cli, dockerhandler.RunOptions{
		Image:  listing.Image,
		Cmd:    listing.Command,
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

func RunDownloadAction(ctx context.Context, cli client.APIClient, model AIModel, handler ModelHandler, config config.Configuration, plat platform.Platform, publish func(e ModelDownloadEvent)) error {
	downloadPath := config.CustomModelsDir()
	if err := downloadPath.MkdirAll(); err != nil {
		return fmt.Errorf("cannot create download location %q: %w", downloadPath, err)
	}

	binds, volumeEnv := ResolveVolumes(handler.Volumes, map[string]string{
		"ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR": downloadPath.String(),
	})

	var envVars map[string]string
	if model.Deployment != nil {
		envVars = model.Deployment.VariablesForPlatform(plat.BoardName)
	}

	env := make([]string, 0, len(envVars)+len(volumeEnv))
	env = append(env, volumeEnv...)
	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	slog.Debug("running download action", "model", model.ID)
	return dockerhandler.Run(ctx, cli, dockerhandler.RunOptions{
		Image: handler.Image,
		Cmd:   handler.Actions.Download,
		Binds: binds,
		Env:   env,
		LineCallback: func(line string) {
			slog.Debug("download line", "model", model.ID, "line", line)
			parseDownloadHandlerLine(line, publish)
		},
		Stderr: io.Discard,
	})
}

type ModelInstallEventType string

const (
	ModelInstallEventStart    ModelInstallEventType = "start"
	ModelInstallEventUpdate   ModelInstallEventType = "update"
	ModelInstallEventComplete ModelInstallEventType = "complete"
	ModelInstallEventInfo     ModelInstallEventType = "info"
	ModelInstallEventError    ModelInstallEventType = "error"
	ModelInstallEventDone     ModelInstallEventType = "done"
)

type ModelDownloadEvent struct {
	ModelID     string                `json:"model_id"`
	Type        ModelInstallEventType `json:"type"`
	Description string                `json:"description,omitempty"`
	Current     int64                 `json:"current,omitempty"`
	Total       int64                 `json:"total,omitempty"`
	Unit        string                `json:"unit,omitempty"`
	Percentage  string                `json:"percentage,omitempty"`
	Artifacts   []string              `json:"artifacts,omitempty"`
}

func parseDownloadHandlerLine(line string, publish func(ModelDownloadEvent)) {
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
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		slog.Debug("non-JSON stdout from handler", "line", line)
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
	publish(ModelDownloadEvent{
		Type:        eventType,
		Description: raw.Description,
		Current:     raw.Current,
		Total:       raw.Total,
		Unit:        raw.Unit,
		Percentage:  raw.Percentage,
		Artifacts:   raw.Artifacts,
	})
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

type rawListingEntry struct {
	Image   string   `yaml:"image"`
	Volumes []string `yaml:"volumes"`
	Command []string `yaml:"command"`
}

type rawHandlersList struct {
	Listing  rawListingEntry              `yaml:"listing"`
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
