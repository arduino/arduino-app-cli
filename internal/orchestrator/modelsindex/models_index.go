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
	"log/slog"
	"slices"
	"strconv"

	"github.com/docker/docker/client"

	"github.com/arduino/arduino-app-cli/internal/dockerhandler"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex/custommodel"
	"github.com/arduino/arduino-app-cli/internal/platform"

	"github.com/arduino/go-paths-helper"
	"github.com/goccy/go-yaml"
	"go.bug.st/f"
)

type assetsModelList struct {
	Models []map[string]AIModel `yaml:"models"`
}

func (b *assetsModelList) UnmarshalYAML(unmarshal func(any) error) error {
	type assetsModelListAlias assetsModelList // Trick to avoid infinite recursion
	var raw assetsModelListAlias
	if err := unmarshal(&raw); err != nil {
		return err
	}
	b.Models = make([]map[string]AIModel, len(raw.Models))
	for i := range raw.Models {
		for key, model := range raw.Models[i] {
			model.ID = key
			b.Models[i] = map[string]AIModel{key: model}
		}
	}
	return nil
}

type PlatformDeploymentConfig struct {
	Variables map[string]string `yaml:"variables"`
}

type ModelDeployment struct {
	Handler   string                                `yaml:"handler"`
	PreLoaded bool                                  `yaml:"pre-loaded"`
	Variables []map[string]PlatformDeploymentConfig `yaml:"platforms,omitempty"`
}

func (d *ModelDeployment) VariablesForPlatform(boardName string) map[string]string {
	for _, entry := range d.Variables {
		if cfg, ok := entry[boardName]; ok {
			return cfg.Variables
		}
	}
	return nil
}

type AIModel struct {
	ID                string            `yaml:"-"`
	ModelFolderPath   *paths.Path       `yaml:"-"`
	Name              string            `yaml:"name"`
	ModuleDescription string            `yaml:"description"`
	Runner            string            `yaml:"runner"`
	Bricks            []BrickConfig     `yaml:"bricks,omitempty"`
	ModelLabels       []string          `yaml:"model_labels,omitempty"`
	Metadata          map[string]string `yaml:"metadata,omitempty"`
	IsInternal        bool              `yaml:"-"`
	Installed         bool              `yaml:"-"`
	Size              uint64            `yaml:"-"`
	SupportedBoards   []string          `yaml:"supported_boards,omitempty"`
	Deployment        *ModelDeployment  `yaml:"deployment,omitempty"`
}

type BrickConfig struct {
	ID                 string            `yaml:"id"`
	ModelConfiguration map[string]string `yaml:"model_configuration"`
}

type ModelsIndex struct {
	InternalModels []AIModel
	modelsDir      *paths.Path
	Handlers       *HandlersIndex
	cli            client.APIClient
	plat           platform.Platform
}

func (m *ModelsIndex) GetModels(ctx context.Context) []AIModel {
	return m.loadModels(ctx)
}

func (m *ModelsIndex) GetModelsByBricks(ctx context.Context, bricks []string) []AIModel {
	return m.filterByBricks(bricks)
}

// FindModelByID returns the model with the given ID without checking installed status.
func (m *ModelsIndex) FindModelByID(id string) (*AIModel, bool) {
	models := m.getModels()
	idx := slices.IndexFunc(models, func(v AIModel) bool { return v.ID == id })
	if idx == -1 {
		return nil, false
	}
	return &models[idx], true
}

// GetModelByID returns the model with the given ID and populates its Installed and Size fields.
func (m *ModelsIndex) GetModelByID(ctx context.Context, id string) (*AIModel, bool, error) {
	model, ok := m.FindModelByID(id)
	if !ok {
		return nil, false, nil
	}
	if model.IsInternal && model.Deployment != nil && model.Deployment.PreLoaded {
		if sizeMBStr, ok := model.Metadata["model_size_mb"]; ok {
			if sizeMB, err := strconv.ParseFloat(sizeMBStr, 64); err == nil && sizeMB > 0 {
				model.Size = uint64(sizeMB * 1024 * 1024)
			}
		}
	} else if model.IsInternal {
		installed, err := m.modelInstalled(ctx, *model)
		if err != nil {
			return nil, false, fmt.Errorf("cannot determine install status for model %q: %w", id, err)
		}
		model.Installed = installed
		if model.Installed {
			model.Size = m.modelSize(ctx, *model)
		}

	}

	return model, true, nil
}

func (m *ModelsIndex) GetModelsByBrick(brickName string) []AIModel {
	var matches []AIModel
	for _, model := range m.loadModels(context.Background()) {
		if slices.ContainsFunc(model.Bricks, func(b BrickConfig) bool { return b.ID == brickName }) {
			matches = append(matches, model)
		}
	}
	return matches
}

func (m *ModelsIndex) filterByBricks(bricks []string) []AIModel {
	var modelList []AIModel
	for _, model := range m.loadModels(context.Background()) {
		if slices.ContainsFunc(model.Bricks, func(brick BrickConfig) bool {
			return slices.Contains(bricks, brick.ID)
		}) {
			modelList = append(modelList, model)
		}
	}
	return modelList
}

func (m *ModelsIndex) loadModels(ctx context.Context) []AIModel {
	models := m.getModels()
	m.setStatus(ctx, models)
	m.setSizes(ctx, models)
	return models
}

func (m *ModelsIndex) getModels() []AIModel {
	eimodels, err := loadCustomModels(m.modelsDir)
	if err != nil {
		slog.Error("cannot load edge impulse custom models", "err", err)
	}
	return append(m.InternalModels, eimodels...)
}

func (m *ModelsIndex) setStatus(ctx context.Context, l []AIModel) []AIModel {
	statuses := m.Handlers.getModelStatus(ctx, m.cli, m.modelsDir, m.plat)
	for i := range l {
		if installed, ok := statuses[l[i].ID]; ok && !l[i].IsInternal {
			l[i].Installed = installed
		}
	}
	return l
}

func (m *ModelsIndex) setSizes(ctx context.Context, l []AIModel) []AIModel {
	sizes := m.Handlers.getModelSizes(ctx, m.cli, l, m.modelsDir, m.plat)
	for i := range l {
		if sizeMBStr, ok := l[i].Metadata["model_size_mb"]; ok {
			if sizeMB, err := strconv.ParseFloat(sizeMBStr, 64); err == nil && sizeMB > 0 {
				l[i].Size = uint64(sizeMB * 1024 * 1024)
			}
		} else if size, ok := sizes[l[i].ID]; ok {
			l[i].Size = size
		}
		// TODO Custom-EI are out from this flow.
	}
	return l
}

// Load constructs a ModelsIndex. Pass the result of LoadHandlers as handlers;
// nil is accepted and disables handler-backed status checks.
func Load(plat platform.Platform, dir *paths.Path, modelsDir *paths.Path, cli client.APIClient, dockerRegistryBase string) (*ModelsIndex, error) {
	if dir == nil && modelsDir == nil {
		return &ModelsIndex{}, errors.New("either dir or modelsDir must be provided")
	}
	models, err := loadInternalModels(dir)
	if err != nil {
		return nil, err
	}

	models = slices.DeleteFunc(models, func(model AIModel) bool {
		return plat.BoardName != "" &&
			len(model.SupportedBoards) != 0 &&
			!slices.Contains(model.SupportedBoards, plat.BoardName)
	})

	handlers, err := loadHandlers(dir, dockerRegistryBase)
	if err != nil {
		return nil, err
	}

	return &ModelsIndex{
		InternalModels: models,
		modelsDir:      modelsDir,
		Handlers:       handlers,
		cli:            cli,
		plat:           plat,
	}, nil
}

func (m *ModelsIndex) modelSize(ctx context.Context, model AIModel) uint64 {
	if sizeMBStr, ok := model.Metadata["model_size_mb"]; ok {
		if sizeMB, err := strconv.ParseFloat(sizeMBStr, 64); err == nil && sizeMB > 0 {
			return uint64(sizeMB * 1024 * 1024)
		}
	}
	if model.Deployment == nil || model.Deployment.Handler == "" {
		return 0
	}
	handler, ok := m.Handlers.GetHandlerByID(model.Deployment.Handler)
	if !ok || len(handler.Actions.Info) == 0 {
		return 0
	}
	size, err := runInfoAction(ctx, m.cli, handler, model, m.modelsDir, m.plat)
	if err != nil {
		slog.Warn("cannot get model size", "model", model.ID, "err", err)
		return 0
	}
	return size
}

func (m *ModelsIndex) modelInstalled(ctx context.Context, model AIModel) (bool, error) {
	if model.Deployment == nil || model.Deployment.Handler == "" {
		return false, fmt.Errorf("model %q has no deployment handler", model.ID)
	}
	handler, ok := m.Handlers.GetHandlerByID(model.Deployment.Handler)
	if !ok {
		return false, fmt.Errorf("handler %q not found for model %q", model.Deployment.Handler, model.ID)
	}
	var envVars map[string]string
	envVars = model.Deployment.VariablesForPlatform(m.plat.BoardName)
	return isModelDownloaded(ctx, m.cli, model, handler, m.modelsDir, envVars)
}

func isModelDownloaded(ctx context.Context, cli client.APIClient, model AIModel, handler ModelHandler, downloadPath *paths.Path, envVars map[string]string) (bool, error) {
	binds, volumeEnv := ResolveVolumes(handler.Volumes, map[string]string{
		"ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR": downloadPath.String(),
	})
	env := volumeEnv
	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	var hasInfoEvent bool
	err := dockerhandler.Run(ctx, cli, dockerhandler.RunOptions{
		Image: handler.Image,
		Cmd:   handler.Actions.Check,
		Binds: binds,
		Env:   env,
		LineCallback: func(line string) {
			var out struct {
				Event string `json:"event"`
			}
			if jsonErr := json.Unmarshal([]byte(line), &out); jsonErr == nil && out.Event == "error" {
				hasInfoEvent = true
			}
		},
	})
	if err != nil {
		if hasInfoEvent {
			slog.Debug("model not installed", "model", model.ID)
			return false, nil
		} else {
			return false, fmt.Errorf("model check failed for %q: %w", model.ID, err)
		}
	}
	return true, nil
}

func loadInternalModels(dir *paths.Path) ([]AIModel, error) {
	if dir == nil {
		// skip loading internal models
		return []AIModel{}, nil
	}
	content, err := dir.Join("models-list.yaml").ReadFile()
	if err != nil {
		return nil, err
	}

	var list assetsModelList
	if err := yaml.Unmarshal(content, &list); err != nil {
		return nil, err
	}

	models := make([]AIModel, len(list.Models))
	for i, modelMap := range list.Models {
		for id, model := range modelMap {
			model.ID = id
			model.IsInternal = true
			models[i] = model
		}
	}
	return models, nil
}

func loadCustomModels(dir *paths.Path) ([]AIModel, error) {
	if dir == nil {
		// skip loading custom models
		return []AIModel{}, nil
	}
	models := make([]AIModel, 0)
	res, err := dir.ReadDirRecursiveFiltered(func(file *paths.Path) bool {
		if file.Join("model.yaml").NotExist() {
			// let's continue scanning, the model can be in a subfolder
			return true
		}
		return false
	}, paths.FilterDirectories())
	if err != nil {
		slog.Error("unable to list models", slog.String("error", err.Error()), "dir", dir)
		return models, err
	}
	for _, file := range res {
		m, err := custommodel.Load(file)
		if err != nil {
			slog.Warn("unable to load custom model", slog.String("error", err.Error()), "path", file)
			continue // FIXME: collect broken models
		}

		var modelSizeMB uint64
		if modelFileInfo, err := m.FullPath.Join("model.eim").Stat(); err != nil {
			slog.Warn("unable to stat custom model file", slog.String("error", err.Error()), "path", m.FullPath.Join("model.eim"))
		} else {
			modelSizeBytes := modelFileInfo.Size()
			if modelSizeBytes > 0 {
				sizeBytes := uint64(modelSizeBytes)
				modelSizeMB = (sizeBytes + (1024*1024 - 1)) / (1024 * 1024)
			}
		}

		models = append(models, AIModel{
			ID:                m.ModelDescriptor.ID,
			Name:              m.ModelDescriptor.Name,
			ModuleDescription: m.ModelDescriptor.Description,
			Bricks: f.Map(m.ModelDescriptor.Bricks, func(b custommodel.BrickConfig) BrickConfig {
				return BrickConfig(b)
			}),
			Metadata:        m.ModelDescriptor.Metadata,
			ModelFolderPath: m.FullPath,
			IsInternal:      false,
			Installed:       true,
			Size:            modelSizeMB,
		})
	}

	return models, nil
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

	var listing *ListingConfig
	if raw.Listing.Image != "" {
		listing = &ListingConfig{
			Image:   resolveImage(raw.Listing.Image, registryBase),
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
				Image:   resolveImage(entry.Image, registryBase),
				Volumes: entry.Volumes,
				Actions: actions,
			}
		}
	}

	return &HandlersIndex{handlers: handlers, listing: listing}, nil
}
