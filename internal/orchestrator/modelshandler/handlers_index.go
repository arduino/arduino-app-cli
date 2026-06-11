package modelshandler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/arduino/go-paths-helper"
	"github.com/docker/docker/client"
	"github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/dockerhandler"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

type HandlersIndex struct {
	handlers map[string]ModelHandler

	downloadPath *paths.Path // read from env ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR  default  /home/arduino/.arduino-app-cli/models)
	cli          client.APIClient
}

type ModelHandler struct {
	ID      string
	Image   string
	Volumes []string
	Actions HandlerActions
}

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

func Load(dir *paths.Path, registryBase string, cli client.APIClient) (*HandlersIndex, error) {
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
				ID: id,
				// image: ${DOCKER_REGISTRY_BASE:-ghcr.io/arduino/}app-bricks/models-downloader:0.10.0
				Image:   resolveImage(entry.Image, registryBase),
				Volumes: entry.Volumes,
				Actions: actions,
			}
		}
	}
	// TODO get from the config
	return &HandlersIndex{handlers: handlers, downloadPath: paths.New("/home/arduino/.arduino-app-cli/models"), cli: cli}, nil
}

func (h *HandlersIndex) ListModels(ctx context.Context, plat platform.Platform) (map[string]bool, error) {
	var env []string
	if plat.BoardName != "" {
		env = append(env, fmt.Sprintf("board=%s", plat.BoardName))
	}

	var buf bytes.Buffer
	err := dockerhandler.Run(ctx, h.cli, dockerhandler.RunOptions{
		// TODO: get the listing image and the "listing" section of the yaml
		Image:  "ghcr.io/arduino/app-bricks/models-downloader:0.10.0",
		Cmd:    []string{"/app/list_models.sh"},
		Binds:  []string{fmt.Sprintf("%s:/models", h.downloadPath.String())},
		Env:    env,
		Stdout: &buf,
	})
	if err != nil {
		return nil, fmt.Errorf("list action: %w", err)
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

	type handlerModelListOutput struct {
		Event  string              `json:"event"`
		Models []handlerModelEntry `json:"models"`
	}

	var res handlerModelListOutput
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		return nil, fmt.Errorf("parsing list output: %w", err)
	}

	var result = make(map[string]bool, len(res.Models))
	for _, entry := range res.Models {
		result[entry.ID] = entry.Installed
	}
	return result, nil
}

func (h *HandlersIndex) GetHandler(id string) (ModelHandler, bool) {
	handler, ok := h.handlers[id]
	return handler, ok
}

func (h *HandlersIndex) RunCheckAction(ctx context.Context, handler ModelHandler, model *modelsindex.AIModel) bool {
	var envVars map[string]string
	if model.Deployment != nil {
		envVars = model.Deployment.VariablesForPlatform("ventunoq") // TODO get the platform
	}
	env := make([]string, 0, len(envVars))
	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	err := dockerhandler.Run(ctx, h.cli, dockerhandler.RunOptions{
		Image: handler.Image,
		Cmd:   handler.Actions.Check,
		Binds: []string{fmt.Sprintf("%s:/models", h.downloadPath.String())},
		Env:   env,
	})
	if err != nil {
		slog.Debug("model check reported not downloaded", "model", model.ID, "err", err)
		return false
	}

	return true
}

type InfoMsg struct {
	Event       string `json:"event"`
	Description string `json:"description"`
	SizeMB      int64  `json:"size_mb"`
}

func (h *HandlersIndex) RunInfoAction(ctx context.Context, handler ModelHandler, model *modelsindex.AIModel, cb func(InfoMsg)) error {
	var envVars map[string]string
	if model.Deployment != nil {
		envVars = model.Deployment.VariablesForPlatform("ventunoq") // TODO get the platform
	}
	env := make([]string, 0, len(envVars))
	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	err := dockerhandler.Run(ctx, h.cli, dockerhandler.RunOptions{
		Image: handler.Image,
		Cmd:   handler.Actions.Info,
		Binds: []string{fmt.Sprintf("%s:/models", h.downloadPath.String())},
		Env:   env,
		LineCallback: func(line string) {
			var msg InfoMsg
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				slog.Debug("non-JSON stdout from handler", "line", line)
				return
			}
			cb(msg)
		},
	})
	if err != nil {
		slog.Debug("model info reported not available", "model", model.ID, "err", err)
		return fmt.Errorf("info action: %w", err)
	}

	return nil
}

type DownloadMsg struct {
	Event       string   `json:"event"`
	Description string   `json:"description"`
	Current     int64    `json:"current"`
	Total       int64    `json:"total"`
	Unit        string   `json:"unit"`
	Percentage  string   `json:"percentage"`
	Artifacts   []string `json:"artifacts"`
}

func (h *HandlersIndex) RunDownloadAction(ctx context.Context, handler ModelHandler, model *modelsindex.AIModel, cb func(DownloadMsg)) error {
	var envVars map[string]string
	if model.Deployment != nil {
		envVars = model.Deployment.VariablesForPlatform("ventunoq") // TODO get the platform
	}
	env := make([]string, 0, len(envVars))
	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	err := dockerhandler.Run(ctx, h.cli, dockerhandler.RunOptions{
		Image: handler.Image,
		Cmd:   handler.Actions.Download,
		Binds: []string{fmt.Sprintf("%s:/models", h.downloadPath.String())},
		Env:   env,
		LineCallback: func(line string) {
			var msg DownloadMsg
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				slog.Debug("non-JSON stdout from handler", "line", line)
				return
			}
			cb(msg)
		},
	})
	if err != nil {
		slog.Debug("model download reported not available", "model", model.ID, "err", err)
		return fmt.Errorf("download action: %w", err)
	}

	return nil
}

type DeleteMsg struct {
	Event       string `json:"event"`
	Description string `json:"description"`
}

// echo "{\"event\": \"error\", \"description\": \"Failed to remove model: ${model_directory}\"}"
// echo "{\"event\": \"info\", \"description\": \"Model removed: ${model_directory}\"}"

func (h *HandlersIndex) RunRemoveAction(ctx context.Context, handler ModelHandler, model *modelsindex.AIModel, cb func(DeleteMsg)) error {
	var envVars map[string]string
	if model.Deployment != nil {
		envVars = model.Deployment.VariablesForPlatform("ventunoq") // TODO get the platform
	}
	env := make([]string, 0, len(envVars))
	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	err := dockerhandler.Run(ctx, h.cli, dockerhandler.RunOptions{
		Image: handler.Image,
		Cmd:   handler.Actions.Delete,
		Binds: []string{fmt.Sprintf("%s:/models", h.downloadPath.String())},
		Env:   env,
		LineCallback: func(line string) {
			var msg DeleteMsg
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				slog.Debug("non-JSON stdout from handler", "line", line)
				return
			}
			cb(msg)
		},
	})
	if err != nil {
		slog.Debug("model remove reported not available", "model", model.ID, "err", err)
		return fmt.Errorf("remove action: %w", err)
	}

	return nil
}

// FIXME: given the ${DOCKER_REGISTRY_BASE:-ghcr.io/arduino/}app-bricks/models-downloader:0.10.0
// this are the steps:
// 3) if env variable is set use it, otherwise use the default value (ghcr.io/arduino/)
// resolveImage replaces a ${VAR:-default} prefix in the image string with registryBase.
func resolveImage(raw, registryBase string) string {
	return registryBase + raw
}
