// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package modelsindex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"slices"

	composetmpl "github.com/compose-spec/compose-go/v2/template"
	"github.com/docker/docker/client"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/dockerhelper"
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

// resolveVars substitutes compose-style ${VAR} and ${VAR:-default} placeholders
// in raw using the provided vars map. Unknown variables are left unchanged.
func ResolveVars(raw string, vars map[string]string) string {
	result, err := composetmpl.Substitute(raw, func(key string) (string, bool) {
		v, ok := vars[key]
		return v, ok
	})
	if err != nil {
		slog.Warn("cannot resolve template variables", "raw", raw, "err", err)
		return raw
	}
	return result
}

// ResolveVarsSlice applies ResolveVars to each string in raws and returns a new slice with the results.
func ResolveVarsSlice(raws []string, vars map[string]string) []string {
	return f.Map(raws, func(v string) string {
		return ResolveVars(v, vars)
	})
}

type ListingConfig struct {
	Image   string
	Volumes []string
	Command []string
}

type HandlersIndex struct {
	handlers  map[string]ModelHandler
	listing   *ListingConfig
	configEnv map[string]string
}

func (h *HandlersIndex) GetHandlerByID(id string) (ModelHandler, bool) {
	handler, ok := h.handlers[id]
	return handler, ok
}

func (h *HandlersIndex) GetListingConfig() *ListingConfig {
	return h.listing
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

var (
	ErrNotFound            = errors.New("model not found")
	ErrConflict            = errors.New("can't delete the model")
	ErrCannotRemoveModel   = errors.New("cannot remove an internal model")
	ErrInsufficientStorage = errors.New("insufficient storage to install the model")
	ErrIncompleteImpulse   = errors.New("impulse not ready for deployment")
)

func (h *HandlersIndex) getModelsInfo(ctx context.Context, cli client.APIClient, models []AIModel, plat platform.Platform) ([]AIModel, error) {
	if h == nil || h.listing == nil {
		slog.Warn("handlers index or listing config is nil, cannot get model info")
		return models, nil
	}
	entries, err := runListAction(ctx, cli, h.listing, plat, h.configEnv)
	if err != nil {
		return models, fmt.Errorf("cannot list models: %w", err)
	}
	// Cloning! this works because we are updating only the Installed and Size fields.
	modelsInfo := slices.Clone(models)
	dryIndex := make(map[string]int, len(models))
	for i, m := range models {
		dryIndex[m.ID] = i
	}
	for _, entry := range entries {
		i, ok := dryIndex[entry.ID]
		if !ok {
			continue
		}

		modelsInfo[i].Installed = entry.Installed
		if entry.Installed && entry.DiskSizeMB != nil && *entry.DiskSizeMB > 0 {
			modelsInfo[i].Size = uint64(*entry.DiskSizeMB * 1024 * 1024)
		} else if entry.ModelSizeMB != nil && *entry.ModelSizeMB > 0 {
			modelsInfo[i].Size = uint64(*entry.ModelSizeMB * 1024 * 1024)
		}
	}
	return modelsInfo, nil
}

func runInfoAction(ctx context.Context, cli client.APIClient, handler ModelHandler, model AIModel, plat platform.Platform, configEnv map[string]string) (uint64, error) {
	var env []string
	if plat.BoardName != "" {
		env = append(env, fmt.Sprintf("board=%s", plat.BoardName))
	}
	var envVars map[string]string
	if model.Deployment != nil {
		envVars = model.Deployment.VariablesForPlatform(plat.BoardName)
	}
	binds := ResolveVarsSlice(handler.Volumes, configEnv)
	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	var size uint64
	err := dockerhelper.Run(ctx, cli, dockerhelper.RunOptions{
		Image: ResolveVars(handler.Image, configEnv),
		Cmd:   handler.Actions.Info,
		Binds: ResolveVarsSlice(binds, configEnv),
		Env:   env,
		Stdout: f.NewCallbackWriter(func(line string) {
			var out struct {
				Event  string  `json:"event"`
				SizeMB float64 `json:"size_mb"`
			}
			if jsonErr := json.Unmarshal([]byte(line), &out); jsonErr == nil && out.Event == "stat" && out.SizeMB > 0 {
				size = uint64(out.SizeMB * 1024 * 1024)
			}
		}),
	})
	if err != nil {
		return 0, fmt.Errorf("info action: %w", err)
	}
	return size, nil
}

func runListAction(ctx context.Context, cli client.APIClient, listing *ListingConfig, plat platform.Platform, configEnv map[string]string) ([]handlerModelEntry, error) {
	var env []string
	if plat.BoardName != "" {
		env = append(env, fmt.Sprintf("BOARD_NAME=%s", plat.BoardName))
	}

	slog.Debug("running list action", "image", listing.Image)

	var buf bytes.Buffer
	err := dockerhelper.Run(ctx, cli, dockerhelper.RunOptions{
		Image:  ResolveVars(listing.Image, configEnv),
		Cmd:    listing.Command,
		Binds:  ResolveVarsSlice(listing.Volumes, configEnv),
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

func Download(ctx context.Context, modelsIndex *ModelsIndex, cli client.APIClient, model AIModel, plat platform.Platform, publish func(e ModelDownloadEvent)) error {

	handler, ok := modelsIndex.Handlers.GetHandlerByID(model.Deployment.Handler)
	if !ok {
		return fmt.Errorf("handler %q not found for model %q", model.Deployment.Handler, model.ID)
	}

	envVars := model.Deployment.VariablesForPlatform(plat.BoardName)
	maps.Insert(envVars, maps.All(modelsIndex.Handlers.configEnv))

	env := make([]string, 0, len(envVars))
	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return dockerhelper.Run(ctx, cli, dockerhelper.RunOptions{
		Image: ResolveVars(handler.Image, envVars),
		Cmd:   handler.Actions.Download,
		Binds: ResolveVarsSlice(handler.Volumes, envVars),
		Env:   env,
		Stdout: f.NewCallbackWriter(func(line string) {
			slog.Debug("download line", "model", model.ID, "line", line)
			// TODO: unify
			parseDownloadHandlerLine(line, publish)
		}),
		Stderr: io.Discard,
	})
}

func deleteInternalModel(ctx context.Context, cli client.APIClient, model AIModel, handler ModelHandler, plat platform.Platform, configEnv map[string]string) error {

	if model.Deployment == nil || model.Deployment.Handler == "" {
		return fmt.Errorf("model %q has no deployment handler", model.ID)
	}

	envVars := model.Deployment.VariablesForPlatform(plat.BoardName)
	maps.Insert(envVars, maps.All(configEnv)) // include config env vars for template resolution

	env := make([]string, 0, len(envVars))
	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	slog.Debug("running delete action", "model", model.ID)
	return dockerhelper.Run(ctx, cli, dockerhelper.RunOptions{
		Image:  ResolveVars(handler.Image, envVars),
		Cmd:    handler.Actions.Delete,
		Binds:  ResolveVarsSlice(handler.Volumes, envVars),
		Env:    env,
		Stdout: io.Discard,
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

func (h *HandlersIndex) GetDockerImages() []string {
	if h == nil {
		slog.Warn("handlers index is nil, cannot get model handler images")
		return []string{}
	}

	images := make(map[string]struct{})
	for _, handler := range h.handlers {
		image := ResolveVars(handler.Image, h.configEnv)
		images[image] = struct{}{}
	}

	if h.listing != nil && h.listing.Image != "" {
		image := ResolveVars(h.listing.Image, h.configEnv)
		images[image] = struct{}{}
	}

	return slices.Collect(maps.Keys(images))
}
