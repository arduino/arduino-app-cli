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
	"path/filepath"
	"slices"
	"strconv"
	"strings"
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

	IsInternal bool   `yaml:"-"`
	Installed  bool   `yaml:"-"`
	Size       uint64 `yaml:"-"`
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
	InternalModels []AIModel
	modelsDir      *paths.Path
	Handlers       *HandlersIndex
	cli            client.APIClient
	plat           platform.Platform
}

func (m *ModelsIndex) GetModels(ctx context.Context) []AIModel {
	models := m.loadDryModels()
	if m.Handlers != nil {
		var err error
		models, err = m.Handlers.getModelsInfo(ctx, m.cli, models)
		if err != nil {
			slog.Warn("cannot get models info", "err", err)
		}
	}

	// trust the sentinel files instead of the handler list action.
	for i := range models {
		installed, err := m.modelInstalled(ctx, models[i], m.cli)
		if err != nil {
			slog.Warn("cannot determine install status for model", "model", models[i].ID, "err", err)
			continue
		}
		models[i].Installed = installed
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
	if model.IsInternal && model.Deployment != nil && !model.Deployment.PreLoaded {
		// TODO we should have a single method that do the check and get the info
		installed, err := m.modelInstalled(ctx, model, m.cli)
		if err != nil {
			return nil, fmt.Errorf("cannot determine install status for model %q: %w", id, err)
		}
		model.Installed = installed
		if model.Installed {
			// TODO : we should return an error if the size cannot be determined
			model.Size = m.modelSize(ctx, model)
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
	eimodels, err := loadCustomModels(m.modelsDir)
	if err != nil {
		slog.Error("cannot load edge impulse custom models", "err", err)
	}
	models := slices.Clone(m.InternalModels)
	return append(models, eimodels...)
}

// Load constructs a ModelsIndex. Pass the result of LoadHandlers as handlers;
// nil is accepted and disables handler-backed status checks.
func Load(plat platform.Platform, dir *paths.Path, modelsDir *paths.Path, cli client.APIClient, cfg config.Configuration) (*ModelsIndex, error) {
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
	size, err := runInfoAction(ctx, m.cli, handler, model, m.plat, m.Handlers.configEnv)
	if err != nil {
		slog.Warn("cannot get model size", "model", model.ID, "err", err)
		return 0
	}
	return size
}

func sentinelPath(modelsDir *paths.Path, modelID string) *paths.Path {
	return modelsDir.Join(fmt.Sprintf(".%s.downloaded.json", modelID))
}

func (m *ModelsIndex) modelInstalled(_ context.Context, model AIModel, _ client.APIClient) (bool, error) {
	return sentinelPath(m.modelsDir, model.ID).Exist(), nil
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
			ID:          m.ModelDescriptor.ID,
			Name:        m.ModelDescriptor.Name,
			Description: m.ModelDescriptor.Description,
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

	volumes := ResolveVarsSlice(handler.Volumes, envVars)

	var artifacts []string
	if err := dockerhelper.Run(ctx, cli, dockerhelper.RunOptions{
		Image: ResolveVars(handler.Image, envVars),
		Cmd:   handler.Actions.Download,
		Binds: volumes,
		Env:   envVars,
		Stdout: f.NewCallbackWriter(func(line string) {
			slog.Debug("download line", "model", model.ID, "line", line)
			if a := parseDownloadHandlerLine(line, publish); a != nil {
				artifacts = a
			}
		}),
		Stderr: io.Discard,
	}); err != nil {
		return fmt.Errorf("download action failed for model %q: %w", model.ID, err)
	}

	if len(artifacts) == 0 {
		return fmt.Errorf("download action for model %q did not produce any artifacts", model.ID)
	}

	resolved := resolveArtifactPaths(volumes, artifacts)
	hostPaths := make([]string, 0, len(resolved))
	for _, p := range resolved {
		hostPaths = append(hostPaths, p.String())
	}
	payload, err := json.Marshal(struct {
		Artifacts []string `json:"artifacts"`
	}{Artifacts: hostPaths})
	if err != nil {
		return fmt.Errorf("cannot encode sentinel for model %q: %w", model.ID, err)
	}

	sentinelFile := sentinelPath(m.modelsDir, model.ID)
	if err := sentinelFile.WriteFile(payload); err != nil {
		return fmt.Errorf("cannot write sentinel file for model %q: %w", model.ID, err)
	}

	return nil
}

// resolveArtifactPaths translates artifact paths emitted by a handler
// (container-absolute or relative to the volume mount) into absolute host paths.
func resolveArtifactPaths(bindVolumes []string, artifacts []string) []*paths.Path {
	type bind struct{ host, container string }
	binds := make([]bind, 0, len(bindVolumes))
	for _, bv := range bindVolumes {
		host, container, ok := strings.Cut(bv, ":")
		if !ok || host == "" || container == "" {
			continue
		}
		binds = append(binds, bind{host: host, container: container})
	}

	result := make([]*paths.Path, 0, len(artifacts))
	for _, art := range artifacts {
		var resolved string
		switch {
		case filepath.IsAbs(art):
			for _, b := range binds {
				if rel, ok := strings.CutPrefix(art, b.container); ok {
					resolved = filepath.Join(b.host, rel)
					break
				}
			}
			if resolved == "" {
				resolved = art
			}
		case len(binds) > 0:
			resolved = filepath.Join(binds[0].host, art)
		default:
			resolved = art
		}
		result = append(result, paths.New(resolved))
	}
	return result
}

func (m *ModelsIndex) Delete(ctx context.Context, dockerClient command.Cli, platform platform.Platform, model AIModel) error {
	if model.Deployment != nil && model.Deployment.Handler != "" {
		handler, ok := m.Handlers.GetHandlerByID(model.Deployment.Handler)
		if !ok {
			return fmt.Errorf("handler %q not found for model %q", model.Deployment.Handler, model.ID)
		}
		if err := deleteInternalModel(ctx, dockerClient.Client(), model, handler, platform, m.Handlers.configEnv); err != nil {
			return fmt.Errorf("delete action: %w", err)
		}
		sentinel := sentinelPath(m.modelsDir, model.ID)
		if sentinel.Exist() {
			if err := sentinel.Remove(); err != nil {
				return fmt.Errorf("cannot remove sentinel file for model %q: %w", model.ID, err)
			}
		}
	} else {
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
