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
	"strconv"
	"syscall"

	"github.com/docker/cli/cli/command"
	"github.com/docker/docker/client"
	"github.com/shirou/gopsutil/v4/disk"

	"github.com/arduino/arduino-app-cli/internal/dockerhelper"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
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
			if cfg.Variables == nil {
				return map[string]string{}
			}
			return cfg.Variables
		}
	}
	return map[string]string{}
}

type AIModel struct {
	ID              string            `yaml:"-"`
	ModelFolderPath *paths.Path       `yaml:"-"`
	Name            string            `yaml:"name"`
	Description     string            `yaml:"description"`
	Runner          string            `yaml:"runner"`
	Bricks          []BrickConfig     `yaml:"bricks,omitempty"`
	ModelLabels     []string          `yaml:"model_labels,omitempty"`
	Metadata        map[string]string `yaml:"metadata,omitempty"`
	SupportedBoards []string          `yaml:"supported_boards,omitempty"`
	Deployment      *ModelDeployment  `yaml:"deployment,omitempty"`

	IsBuiltIn bool        `yaml:"-"` // a model is considered built-in if it is in the models-list.yaml and the "pre-loaded" flag is true
	Status    ModelStatus `yaml:"-"`
	Size      uint64      `yaml:"-"`
}

type ModelStatus string

const (
	InstalledStatus    ModelStatus = "installed"
	NotInstalledStatus ModelStatus = "not-installed"
)

func (s ModelStatus) AllowedStatuses() []ModelStatus {
	return []ModelStatus{InstalledStatus, NotInstalledStatus}
}

type AIModelLite struct {
	ID          string
	Name        string
	Description string
}

type BrickConfig struct {
	ID                 string            `yaml:"id"`
	ModelConfiguration map[string]string `yaml:"model_configuration"`
}

type ModelsIndex struct {
	InternalModels  []AIModel
	modelsDir       *paths.Path
	customModelsDir *paths.Path
	Handlers        *HandlersIndex
	cli             client.APIClient
	plat            platform.Platform
}

func (m *ModelsIndex) GetModels(ctx context.Context) []AIModel {
	models := m.loadDryModels()
	if m.Handlers != nil {
		models, err := m.Handlers.getModelsInfo(ctx, m.cli, models)
		if err != nil {
			slog.Warn("cannot get models info", "err", err)
		}
		return models
	}
	return models
}

// GetModelByID returns the model with the given ID and populates its Installed and Size fields.
func (m *ModelsIndex) GetModelByID(ctx context.Context, id string) (*AIModel, error) {
	models := m.loadDryModels()
	idx := slices.IndexFunc(models, func(v AIModel) bool { return v.ID == id })
	if idx == -1 {
		return nil, nil
	}
	model := models[idx]
	if model.Deployment != nil && model.Deployment.Handler != "" && !model.Deployment.PreLoaded { // non-preloaded internal models: determine actual install status
		// TODO we should have a single method that do the check and get the info
		installed, err := m.modelInstalled(ctx, model, m.cli)
		if err != nil {
			return nil, fmt.Errorf("cannot determine install status for model %q: %w", id, err)
		}
		if installed {
			model.Status = InstalledStatus
			// TODO : we should return an error if the size cannot be determined
			model.Size = m.modelSize(ctx, model)
		} else {
			model.Status = NotInstalledStatus
		}
	}

	return &model, nil
}

// GetModelsByBrick returns the models that are associated with the given brick name.
func (m *ModelsIndex) GetModelsByBrick(brickID string) []AIModelLite {
	models := m.loadDryModels()
	matches := make([]AIModelLite, 0, len(models))

	for _, model := range models {
		if slices.ContainsFunc(model.Bricks, func(b BrickConfig) bool { return b.ID == brickID }) {
			matches = append(matches, AIModelLite{
				ID:          model.ID,
				Name:        model.Name,
				Description: model.Description,
			})
		}
	}

	return matches
}

func (m *ModelsIndex) IsModelSupportedByBrick(modelID, brickID string) bool {
	models := m.GetModelsByBrick(brickID)
	return slices.ContainsFunc(models, func(model AIModelLite) bool {
		return model.ID == modelID
	})
}

func (m *ModelsIndex) loadDryModels() []AIModel {
	eiModels, err := loadCustomModels(m.customModelsDir)
	if err != nil {
		slog.Error("cannot load edge impulse custom models", "err", err)
	}
	models := slices.Clone(m.InternalModels)
	return append(models, eiModels...)
}

// Load constructs a ModelsIndex. Pass the result of LoadHandlers as handlers;
// nil is accepted and disables handler-backed status checks.
func Load(plat platform.Platform, dir *paths.Path, modelsDir *paths.Path, customModelsDir *paths.Path, cli client.APIClient, cfg config.Configuration) (*ModelsIndex, error) {
	if dir == nil || modelsDir == nil {
		return &ModelsIndex{}, errors.New("either dir or modelsDir must be provided")
	}

	handlers, err := loadHandlers(dir, modelsDir, cfg, plat)
	if err != nil {
		return nil, err
	}

	models, err := loadInternalModels(dir, handlers)
	if err != nil {
		return nil, err
	}

	models = slices.DeleteFunc(models, func(model AIModel) bool {
		return plat.BoardName != "" &&
			len(model.SupportedBoards) != 0 &&
			!slices.Contains(model.SupportedBoards, plat.BoardName)
	})

	return &ModelsIndex{
		InternalModels:  models,
		customModelsDir: customModelsDir,
		modelsDir:       modelsDir,
		Handlers:        handlers,
		cli:             cli,
		plat:            plat,
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
	size, err := runInfoAction(ctx, m.cli, handler, model, m.plat, m.Handlers.configEnv)
	if err != nil {
		slog.Warn("cannot get model size", "model", model.ID, "err", err)
		return 0
	}
	return size
}

func (m *ModelsIndex) modelInstalled(ctx context.Context, model AIModel, cli client.APIClient) (bool, error) {
	if model.Deployment == nil || model.Deployment.Handler == "" {
		return false, fmt.Errorf("model %q has no deployment handler", model.ID)
	}
	handler, ok := m.Handlers.GetHandlerByID(model.Deployment.Handler)
	if !ok {
		return false, fmt.Errorf("handler %q not found for model %q", model.Deployment.Handler, model.ID)
	}

	envVars := model.Deployment.VariablesForPlatform(m.plat.BoardName)
	maps.Insert(envVars, maps.All(m.Handlers.configEnv))

	var buf bytes.Buffer
	err := dockerhelper.Run(ctx, cli, dockerhelper.RunOptions{
		Image:  ResolveVars(handler.Image, envVars),
		Cmd:    handler.Actions.Check,
		Binds:  ResolveVarsSlice(handler.Volumes, envVars),
		Env:    envVars,
		Stdout: &buf,
	})
	if err != nil {
		return false, fmt.Errorf("model check failed for %q: %w", model.ID, err)
	}

	var out struct {
		Event       string `json:"event"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		return false, fmt.Errorf("model check returned invalid JSON for %q: %w", model.ID, err)
	}
	if out.Event == "error" {
		slog.Debug("model check returned error", "model", model.ID, "description", out.Description)
		return false, nil
	}
	if out.Event == "info" {
		return true, nil
	}
	return false, fmt.Errorf("model check returned unexpected event %q for %q", out.Event, model.ID)
}

func loadInternalModels(dir *paths.Path, handlers *HandlersIndex) ([]AIModel, error) {
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
			model.Status = NotInstalledStatus

			if sizeMBStr, ok := model.Metadata["model_size_mb"]; ok {
				if sizeMB, err := strconv.ParseFloat(sizeMBStr, 64); err == nil && sizeMB > 0 {
					model.Size = uint64(sizeMB * 1024 * 1024)
				}
			}

			if model.Deployment == nil {
				model.IsBuiltIn = true
				model.Status = InstalledStatus
			} else {
				// Handler must be non-empty when pre-loaded is false
				if model.Deployment.Handler == "" && !model.Deployment.PreLoaded {
					return nil, fmt.Errorf("model %q has no handler but is not pre-loaded", model.ID)
				}

				if model.Deployment.Handler != "" {
					_, ok := handlers.GetHandlerByID(model.Deployment.Handler)
					if !ok {
						return nil, fmt.Errorf("handler %q not found for model %q", model.Deployment.Handler, model.ID)
					}
				}

				if model.Deployment.PreLoaded {
					model.IsBuiltIn = true
					model.Status = InstalledStatus
				}
			}
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
			ID:          m.ModelDescriptor.ID,
			Name:        m.ModelDescriptor.Name,
			Description: m.ModelDescriptor.Description,
			Bricks: f.Map(m.ModelDescriptor.Bricks, func(b custommodel.BrickConfig) BrickConfig {
				return BrickConfig(b)
			}),
			Metadata:        m.ModelDescriptor.Metadata,
			ModelFolderPath: m.FullPath,
			IsBuiltIn:       false,
			Status:          InstalledStatus,
			Size:            modelSizeMB,
		})
	}

	return models, nil
}

func (m *ModelsIndex) Download(ctx context.Context, cli client.APIClient, model AIModel, plat platform.Platform, publish func(e StreamMessage)) error {
	if err := hasSufficientDiskSpace(m.modelsDir, model.Size); err != nil {
		return fmt.Errorf("insufficient disk space to download model %q: %w", model.ID, err)
	}

	handler, ok := m.Handlers.GetHandlerByID(model.Deployment.Handler)
	if !ok {
		return fmt.Errorf("handler %q not found for model %q", model.Deployment.Handler, model.ID)
	}

	envVars := model.Deployment.VariablesForPlatform(plat.BoardName)
	maps.Insert(envVars, maps.All(m.Handlers.configEnv))

	return dockerhelper.Run(ctx, cli, dockerhelper.RunOptions{
		Image: ResolveVars(handler.Image, envVars),
		Cmd:   handler.Actions.Download,
		Binds: ResolveVarsSlice(handler.Volumes, envVars),
		Env:   envVars,
		Stdout: f.NewCallbackWriter(func(line string) {
			slog.Debug("download line", "model", model.ID, "line", line)
			parseDownloadHandlerLine(line, publish)
		}),
		Stderr: io.Discard,
	})
}

func (m *ModelsIndex) Delete(ctx context.Context, dockerClient command.Cli, platform platform.Platform, model AIModel) error {
	if model.Deployment != nil && model.Deployment.Handler != "" {
		// Internal model: run the delete action using the handler.
		handler, ok := m.Handlers.GetHandlerByID(model.Deployment.Handler)
		if !ok {
			return fmt.Errorf("handler %q not found for model %q", model.Deployment.Handler, model.ID)
		}
		if err := deleteInternalModel(ctx, dockerClient.Client(), model, handler, platform, m.Handlers.configEnv); err != nil {
			return fmt.Errorf("delete action: %w", err)
		}
	} else {
		// Custom model (e.g. Edge Impulse): remove the model folder directly.
		if model.ModelFolderPath == nil {
			slog.Warn("Cannot remove the model with missing model folder", "id", model.ID)
			return nil
		}
		if err := model.ModelFolderPath.RemoveAll(); err != nil {
			return fmt.Errorf("error removing model folder %s", model.ModelFolderPath.String())
		}
	}
	return nil
}

var ErrInsufficientStorage = errors.New("insufficient storage to install model")

func hasSufficientDiskSpace(path *paths.Path, requiredBytes uint64) error {
	diskStats, err := disk.Usage(path.String())
	if err != nil && !errors.Is(err, syscall.ENOENT) {
		return err
	}
	if diskStats != nil {
		if diskStats.Used+requiredBytes > diskStats.Total {
			return ErrInsufficientStorage
		}
		return nil
	}
	return nil
}
