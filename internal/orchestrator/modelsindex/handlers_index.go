// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package modelsindex

import (
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
	"github.com/goccy/go-yaml"
	"go.bug.st/f"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/dockerhelper"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

type HandlerActions struct {
	Download []string
	// Info is optional: it's used as a fallback to query the *remote*
	// download size when neither the model metadata nor a local
	// download manifest can provide one. The container's check, delete
	// and list actions have been retired in favor of reading the
	// download manifest directly from app-cli, so they are no longer
	// represented here.
	Info []string
}

func (a HandlerActions) validate(id string) error {
	if len(a.Download) == 0 {
		return fmt.Errorf("handler %q: missing required action \"download\"", id)
	}
	return nil
}

type ModelHandler struct {
	ID      string
	Image   string
	Volumes []string
	Actions HandlerActions
}

func loadHandlers(dir *paths.Path, cfg config.Configuration, plat platform.Platform) (*HandlersIndex, error) {
	// TODO : we should add a method on config to return env variables
	if dir == nil {
		return nil, nil
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
				ID:    id,
				Image: entry.Image,
				// FIXME: Only ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR is supported for now. We do not resolve this variable here because the host volume path (including the model repository segment) is only known at runtime and must be finalized before the container starts.
				Volumes: entry.Volumes,
				Actions: actions,
			}
		}
	}

	configEnv := map[string]string{
		"DOCKER_REGISTRY_BASE": cfg.DockerRegistryBase(),
		"BOARD_NAME":           plat.BoardName,
	}
	if modelsDir := cfg.CustomModelsDir(); modelsDir != nil {
		configEnv["ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR"] = modelsDir.String()
	}
	return &HandlersIndex{handlers: handlers, configEnv: configEnv}, nil
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

type HandlersIndex struct {
	handlers  map[string]ModelHandler
	configEnv map[string]string
}

func (h *HandlersIndex) GetHandlerByID(id string) (ModelHandler, bool) {
	handler, ok := h.handlers[id]
	return handler, ok
}

var (
	ErrNotFound            = errors.New("model not found")
	ErrConflict            = errors.New("can't delete the model")
	ErrCannotRemoveModel   = errors.New("cannot remove an internal model")
	ErrInsufficientStorage = errors.New("insufficient storage to install the model")
	ErrIncompleteImpulse   = errors.New("impulse not ready for deployment")
)

func Download(ctx context.Context, modelsIndex *ModelsIndex, cli client.APIClient, model AIModel, plat platform.Platform, publish func(e StreamMessage)) error {

	handler, ok := modelsIndex.Handlers.GetHandlerByID(model.Deployment.Handler)
	if !ok {
		return fmt.Errorf("handler %q not found for model %q", model.Deployment.Handler, model.ID)
	}

	envVars := model.Deployment.VariablesForPlatform(plat.BoardName)
	maps.Insert(envVars, maps.All(modelsIndex.Handlers.configEnv))
	envVars["model_id"] = model.ID

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
			parseDownloadHandlerLine(line, publish)
		}),
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

type MessageType string

const (
	UnknownType  MessageType = ""
	ProgressType MessageType = "progress"
	InfoType     MessageType = "info"
	ErrorType    MessageType = "error"
)

type StreamMessage struct {
	err      string
	data     string
	progress *Progress
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
func (p *StreamMessage) GetData() string        { return p.data }
func (p *StreamMessage) GetError() string       { return p.err }
func (p *StreamMessage) GetProgress() *Progress { return p.progress }
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
			data: "download complete",
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

type rawHandlersList struct {
	Handlers []map[string]rawHandlerEntry `yaml:"handlers"`
}
