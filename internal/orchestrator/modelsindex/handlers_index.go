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
	"maps"
	"slices"
	"time"

	composetmpl "github.com/compose-spec/compose-go/v2/template"
	"github.com/docker/docker/client"
	"github.com/goccy/go-yaml"
	"go.bug.st/f"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/dockerhelper"
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

func loadHandlers(dir *paths.Path, modelsDir *paths.Path, cfg config.Configuration, plat platform.Platform) (*HandlersIndex, error) {
	// TODO : we should add a method on config to return env variables
	configEnv := map[string]string{
		"DOCKER_REGISTRY_BASE": cfg.DockerRegistryBase(),
		"BOARD_NAME":           plat.BoardName,
		"MODELS_PATH":          modelsDir.String(),
	}

	handlersFile := dir.Join("models-handlers.yaml")
	if handlersFile.NotExist() {
		return nil, nil
	}

	content, err := handlersFile.ReadFile()
	if err != nil {
		return nil, err
	}

	var raw rawHandlersList
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return nil, fmt.Errorf("models-handlers.yaml: %w", err)
	}

	var listing *ListingConfig
	if raw.Listing.Image != "" {
		listing = &ListingConfig{
			Image:   raw.Listing.Image,
			Volumes: raw.Listing.Volumes,
			Command: raw.Listing.Command,
		}
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
				Image:   entry.Image,
				Volumes: entry.Volumes,
				Actions: actions,
			}
		}
	}

	return &HandlersIndex{handlers: handlers, listing: listing, configEnv: configEnv}, nil
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

type entryMetadata struct {
	ModelID string            `json:"model_id"`
	Handler string            `json:"handler"` // a handler id, e.g. "hf-handler"
	Inputs  map[string]string `json:"inputs"`
}

type handlerModelEntry struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Handler     string         `json:"handler"`
	Platform    string         `json:"platform"`
	ModelType   string         `json:"model_type"`
	Path        string         `json:"path"`
	Installed   bool           `json:"installed"`
	Downloading bool           `json:"downloading"`   // a download is in progress or was interrupted
	ModelSizeMB *float64       `json:"model_size_mb"` // from yaml metadata
	DiskSizeMB  *float64       `json:"disk_size_mb"`  // actual on-disk size, only when installed
	ModelOrigin string         `json:"model_origin"`
	Metadata    *entryMetadata `json:"download_metadata"`
}

func (e handlerModelEntry) applyStat(m *AIModel) {
	if e.Installed {
		m.Status = InstalledStatus
	} else {
		m.Status = NotInstalledStatus
	}
	m.Downloading = e.Downloading
	if e.Installed && e.DiskSizeMB != nil && *e.DiskSizeMB > 0 {
		m.Size = uint64(*e.DiskSizeMB * 1024 * 1024)
	} else if e.ModelSizeMB != nil && *e.ModelSizeMB > 0 {
		m.Size = uint64(*e.ModelSizeMB * 1024 * 1024)
	}
}

func (h *HandlersIndex) userDownloadModel(entry handlerModelEntry) (AIModel, bool) {
	if entry.ModelOrigin != "user_configured" {
		return AIModel{}, false
	}
	md := entry.Metadata
	if md == nil || md.Handler == "" || len(md.Inputs) == 0 {
		slog.Warn("skipping model with no download record", "model", entry.ID)
		return AIModel{}, false
	}
	if md.ModelID != entry.ID {
		// One record per repository directory, and a repository can hold several
		// quantizations: the record describes whichever downloaded last. Its variables
		// would send a re-download or a delete at the wrong file.
		slog.Warn("skipping model whose download record names another model",
			"model", entry.ID, "record", md.ModelID)
		return AIModel{}, false
	}
	if _, ok := h.GetHandlerByID(md.Handler); !ok {
		slog.Warn("skipping model with unknown handler", "model", entry.ID, "handler", md.Handler)
		return AIModel{}, false
	}
	return AIModel{
		ID:        entry.ID,
		Name:      entry.Name,
		IsBuiltIn: false,
		Bricks:    []BrickConfig{{ID: "arduino:llm"}},
		Deployment: &ModelDeployment{
			Handler: md.Handler,
			Variables: []map[string]PlatformDeploymentConfig{
				{h.configEnv["BOARD_NAME"]: {Variables: md.Inputs}},
			},
		},
	}, true
}

func (h *HandlersIndex) getModelsInfo(ctx context.Context, cli client.APIClient, models []AIModel) ([]AIModel, error) {
	if h == nil || h.listing == nil {
		slog.Warn("handlers index or listing config is nil, cannot get model info")
		return models, nil
	}
	entries, err := runListAction(ctx, cli, h.listing, h.configEnv)
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
		if i, ok := dryIndex[entry.ID]; ok {
			entry.applyStat(&modelsInfo[i])
			continue
		}
		model, ok := h.userDownloadModel(entry)
		if !ok {
			continue
		}
		entry.applyStat(&model)
		modelsInfo = append(modelsInfo, model)
	}
	return modelsInfo, nil
}

func runListAction(ctx context.Context, cli client.APIClient, listing *ListingConfig, configEnv map[string]string) ([]handlerModelEntry, error) {
	slog.Debug("running list action", "image", listing.Image)

	var buf bytes.Buffer
	start := time.Now()
	err := dockerhelper.Run(ctx, cli, dockerhelper.RunOptions{
		Image:  ResolveVars(listing.Image, configEnv),
		Cmd:    listing.Command,
		Binds:  ResolveVarsSlice(listing.Volumes, configEnv),
		Env:    configEnv,
		Stdout: &buf,
	})
	slog.Debug("list action finished", "duration_s", time.Since(start).Seconds())
	if err != nil {
		return nil, fmt.Errorf("list action: %w", err)
	}

	var output handlerModelListOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		return nil, fmt.Errorf("parsing list output: %w", err)
	}

	return output.Models, nil
}

type MessageType string

const (
	UnknownType  MessageType = ""
	ProgressType MessageType = "progress"
	InfoType     MessageType = "info"
	ErrorType    MessageType = "error"
	DoneType     MessageType = "done"
)

type StreamMessage struct {
	err       string
	data      string
	progress  *Progress
	done      string
	artifacts []string
}

type Progress struct {
	Name     string
	Total    int64
	Current  int64
	Progress float32
}

func (p *StreamMessage) IsData() bool           { return p.data != "" }
func (p *StreamMessage) IsError() bool          { return p.err != "" }
func (p *StreamMessage) IsProgress() bool       { return p.progress != nil }
func (p *StreamMessage) IsDone() bool           { return p.done != "" }
func (p *StreamMessage) GetData() string        { return p.data }
func (p *StreamMessage) GetError() string       { return p.err }
func (p *StreamMessage) GetProgress() *Progress { return p.progress }
func (p *StreamMessage) GetDone() string        { return p.done }
func (p *StreamMessage) GetArtifacts() []string { return p.artifacts }
func (p *StreamMessage) GetType() MessageType {
	if p.IsData() {
		return InfoType
	}
	if p.IsProgress() {
		return ProgressType
	}
	if p.IsError() {
		return ErrorType
	}
	if p.IsDone() {
		return DoneType
	}
	return UnknownType
}

func parseDownloadHandlerLine(line string, publish func(StreamMessage)) {
	var raw struct {
		Event       string   `json:"event"`
		Description string   `json:"description"`
		Current     int64    `json:"current"`
		Total       int64    `json:"total"`
		SizeMB      float64  `json:"size_mb"`
		Unit        string   `json:"unit"`
		Artifacts   []string `json:"artifacts"`
	}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		slog.Debug("non-JSON stdout from handler", "line", line)
		return
	}

	switch raw.Event {
	case "start":
		publish(StreamMessage{
			data: raw.Description,
		})
	case "update":
		publish(StreamMessage{
			progress: &Progress{
				Name:     raw.Description,
				Current:  raw.Current,
				Total:    raw.Total,
				Progress: float32(raw.Current) / float32(raw.Total) * 100,
			},
		})
	case "complete":
		publish(StreamMessage{
			done: "download complete",
		})
	case "info":
		// The files the handler wrote: without this the artifacts are dropped and a
		// caller cannot tell which model arrived.
		publish(StreamMessage{
			data:      raw.Description,
			artifacts: raw.Artifacts,
		})
	case "error":
		publish(StreamMessage{
			err: raw.Description,
		})
	default:
		slog.Warn("unknown event from handler", "event", raw.Event, "line", line)
	}
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

func deleteInternalModel(ctx context.Context, cli client.APIClient, model AIModel, handler ModelHandler, plat platform.Platform, configEnv map[string]string) error {
	if model.Deployment == nil || model.Deployment.Handler == "" {
		return fmt.Errorf("model %q has no deployment handler", model.ID)
	}

	envVars := model.Deployment.VariablesForPlatform(plat.BoardName)
	maps.Insert(envVars, maps.All(configEnv)) // include config env vars for template resolution

	slog.Debug("running delete action", "model", model.ID)
	return dockerhelper.Run(ctx, cli, dockerhelper.RunOptions{
		Image:  ResolveVars(handler.Image, envVars),
		Cmd:    handler.Actions.Delete,
		Binds:  ResolveVarsSlice(handler.Volumes, envVars),
		Env:    envVars,
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
}
