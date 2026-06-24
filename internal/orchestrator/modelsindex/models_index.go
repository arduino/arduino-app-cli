// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package modelsindex

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"slices"
	"strconv"

	"github.com/docker/cli/cli/command"
	"github.com/docker/docker/client"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex/custommodel"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex/manifest"
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
	_ = ctx
	models := m.loadDryModels()
	if m.modelsDir == nil {
		return models
	}
	manifests := manifest.Find(m.modelsDir)
	for i := range models {
		model := &models[i]
		if !model.IsInternal || model.Deployment == nil || model.Deployment.PreLoaded {
			continue
		}
		var size uint64
		for _, mf := range manifests {
			if mf.ModelID == model.ID {
				size += mf.TotalSize()
			}
		}
		if size > 0 {
			model.Installed = true
			model.Size = size
		}
	}
	return models
}

func (m *ModelsIndex) GetModelsByBricks(ctx context.Context, bricks []string) []AIModel {
	var modelList []AIModel
	for _, model := range m.GetModels(ctx) {
		if slices.ContainsFunc(model.Bricks, func(brick BrickConfig) bool {
			return slices.Contains(bricks, brick.ID)
		}) {
			modelList = append(modelList, model)
		}
	}
	return modelList
}

// GetModelByID returns the model with the given ID and populates its Installed and Size fields.
func (m *ModelsIndex) GetModelByID(ctx context.Context, id string) (*AIModel, error) {
	_ = ctx
	models := m.loadDryModels()
	idx := slices.IndexFunc(models, func(v AIModel) bool { return v.ID == id })
	if idx == -1 {
		return nil, nil
	}
	model := models[idx]
	if !model.IsInternal || model.Deployment == nil || model.Deployment.PreLoaded || m.modelsDir == nil {
		return &model, nil
	}
	var size uint64
	for _, mf := range manifest.Find(m.modelsDir) {
		if mf.ModelID == model.ID {
			size += mf.TotalSize()
		}
	}
	if size > 0 {
		model.Installed = true
		model.Size = size
	}
	return &model, nil
}

func (m *ModelsIndex) GetModelsByBrick(ctx context.Context, brickName string) []AIModel {
	var matches []AIModel
	for _, model := range m.GetModels(ctx) {
		if slices.ContainsFunc(model.Bricks, func(b BrickConfig) bool { return b.ID == brickName }) {
			matches = append(matches, model)
		}
	}
	return matches
}

func (m *ModelsIndex) loadDryModels() []AIModel {
	eimodels, err := loadCustomModels(m.modelsDir)
	if err != nil {
		slog.Error("cannot load edge impulse custom models", "err", err)
	}
	models := slices.Clone(m.InternalModels)
	return append(models, eimodels...)
}

// Load constructs a ModelsIndex. Pass the result of LoadHandlers as handlers;
// nil is accepted and disables handler-backed status checks.
func Load(plat platform.Platform, dir *paths.Path, modelsDir *paths.Path, cli client.APIClient, dockerRegistryBase string, cfg config.Configuration) (*ModelsIndex, error) {
	if dir == nil && modelsDir == nil {
		return &ModelsIndex{}, errors.New("either dir or modelsDir must be provided")
	}

	handlers, err := loadHandlers(dir, cfg, plat)
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
		InternalModels: models,
		modelsDir:      modelsDir,
		Handlers:       handlers,
		cli:            cli,
		plat:           plat,
	}, nil
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
			model.IsInternal = true
			model.Installed = true

			if sizeMBStr, ok := model.Metadata["model_size_mb"]; ok {
				if sizeMB, err := strconv.ParseFloat(sizeMBStr, 64); err == nil && sizeMB > 0 {
					model.Size = uint64(sizeMB * 1024 * 1024)
				}
			}

			if model.Deployment != nil {
				_, ok := handlers.GetHandlerByID(model.Deployment.Handler)
				if !ok {
					return nil, fmt.Errorf("handler %q not found for model %q", model.Deployment.Handler, model.ID)
				}

				model.Installed = model.Deployment.PreLoaded
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

func Delete(ctx context.Context, m *ModelsIndex, dockerClient command.Cli, plat platform.Platform, model AIModel) error {
	_ = ctx
	_ = dockerClient
	_ = plat

	if model.Deployment != nil && model.Deployment.Handler != "" {
		var removed int
		for _, mf := range manifest.Find(m.modelsDir) {
			if mf.ModelID != model.ID {
				continue
			}
			for _, p := range mf.AbsPaths() {
				if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
					return fmt.Errorf("remove %s: %w", p, err)
				}
			}
			pruneEmptyDirs(paths.New(mf.Dir), m.modelsDir)
			removed++
		}
		if removed == 0 {
			slog.Warn("delete: model is not installed (no manifest)", "id", model.ID)
		}
		return nil
	}

	if model.ModelFolderPath == nil {
		slog.Warn("Cannot remove the model with missing model folder", "id", model.ID)
		return nil
	}
	if err := model.ModelFolderPath.RemoveAll(); err != nil {
		return fmt.Errorf("error removing model folder %s", model.ModelFolderPath.String())
	}
	return nil
}

// pruneEmptyDirs removes dir if empty and walks upwards, stopping at stopAt.
// Failures are best-effort and silently ignored.
func pruneEmptyDirs(dir, stopAt *paths.Path) {
	if dir == nil || stopAt == nil {
		return
	}
	stop := stopAt.Clean().String()
	cur := dir.Clean()
	for cur.String() != stop && cur.String() != "." && cur.String() != "/" {
		entries, err := cur.ReadDir()
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(cur.String()); err != nil {
			return
		}
		cur = cur.Parent()
	}
}
