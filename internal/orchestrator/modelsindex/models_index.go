// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package modelsindex

import (
	"context"
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
	// Downloading comes from the handler's on-disk ".download" marker, so it covers an
	// interrupted download too. TODO(#585): reconcile with AcquireDownload, which guards
	// concurrent runs into one directory but holds no state across a restart.
	Downloading bool `yaml:"-"`
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

// llamacppRepository is the models_repository GGUF models live under, and the only
// directory the handler listing scans for models the catalog does not declare.
const (
	llamacppRepository = "llamacpp"
	hfHandlerID        = "hf-handler"
)

type ModelsIndex struct {
	InternalModels  []AIModel
	modelsDir       *paths.Path
	customModelsDir *paths.Path
	Handlers        *HandlersIndex
	cli             client.APIClient
	plat            platform.Platform
}

// Lookup answers several model queries against at most one listing run. Callers that
// query per brick in a loop should hold one instead of calling the ModelsIndex methods,
// which take a fresh listing each time. Not safe for concurrent use.
type Lookup struct {
	idx    *ModelsIndex
	models []AIModel
	err    error
	loaded bool
}

func (m *ModelsIndex) NewLookup() *Lookup {
	return &Lookup{idx: m}
}

// listing runs the listing on first use only, so a caller whose models are all
// pre-loaded or custom pays no container start at all. A failure is remembered too:
// retrying it per query would mean a container start per question.
func (l *Lookup) listing(ctx context.Context) error {
	if l.loaded {
		return l.err
	}
	l.models, l.err = l.idx.listModels(ctx)
	l.loaded = true
	return l.err
}

func (l *Lookup) ByID(ctx context.Context, id string) (*AIModel, error) {
	if model, ok := l.idx.declaredModel(id); ok {
		return model, nil
	}
	if err := l.listing(ctx); err != nil {
		return nil, fmt.Errorf("cannot determine install status for model %q: %w", id, err)
	}
	idx := slices.IndexFunc(l.models, func(v AIModel) bool { return v.ID == id })
	if idx == -1 {
		return nil, nil
	}
	return &l.models[idx], nil
}

func (l *Lookup) ByBrick(ctx context.Context, brickID string) ([]AIModelLite, error) {
	err := l.listing(ctx)
	matches := make([]AIModelLite, 0, len(l.models))
	for _, model := range l.models {
		if slices.ContainsFunc(model.Bricks, func(b BrickConfig) bool { return b.ID == brickID }) {
			matches = append(matches, AIModelLite{
				ID:          model.ID,
				Name:        model.Name,
				Description: model.Description,
			})
		}
	}
	return matches, err
}

func (l *Lookup) SupportedByBrick(ctx context.Context, modelID, brickID string) (bool, error) {
	// A declared model needs no listing to answer: its bricks come from models-list.yaml.
	if model, ok := l.idx.declaredModel(modelID); ok {
		return slices.ContainsFunc(model.Bricks, func(b BrickConfig) bool { return b.ID == brickID }), nil
	}
	model, err := l.ByID(ctx, modelID)
	if err != nil || model == nil {
		return false, err
	}
	return slices.ContainsFunc(model.Bricks, func(b BrickConfig) bool { return b.ID == brickID }), nil
}

// declaredModel returns a model whose state comes from its declaration rather than from
// disk - pre-loaded, or a custom model - so no handler run can add anything.
func (m *ModelsIndex) declaredModel(id string) (*AIModel, bool) {
	for _, model := range m.loadDryModels() {
		if model.ID == id && (model.Deployment == nil || model.Deployment.PreLoaded) {
			return &model, true
		}
	}
	return nil, false
}

func (m *ModelsIndex) listModels(ctx context.Context) ([]AIModel, error) {
	dryModels := m.loadDryModels()
	if m.Handlers == nil || m.cli == nil {
		return dryModels, nil
	}
	models, err := m.Handlers.getModelsInfo(ctx, m.cli, dryModels)
	if err != nil {
		return dryModels, err
	}
	return models, nil
}

func (m *ModelsIndex) GetModels(ctx context.Context) []AIModel {
	models, err := m.listModels(ctx)
	if err != nil {
		slog.Warn("cannot get models info, falling back to declared models", "err", err)
	}
	return models
}

func (m *ModelsIndex) GetModelByID(ctx context.Context, id string) (*AIModel, error) {
	return m.NewLookup().ByID(ctx, id)
}

// GetModelsByBrick returns the models that are associated with the given brick name.
func (m *ModelsIndex) GetModelsByBrick(ctx context.Context, brickID string) []AIModelLite {
	matches, err := m.NewLookup().ByBrick(ctx, brickID)
	if err != nil {
		slog.Warn("cannot get models info, brick compatibility list may be incomplete", "brick", brickID, "err", err)
	}
	return matches
}

func (m *ModelsIndex) IsModelSupportedByBrick(ctx context.Context, modelID, brickID string) (bool, error) {
	return m.NewLookup().SupportedByBrick(ctx, modelID, brickID)
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

// DownloadByURL fetches a model no models-list.yaml entry declares, named by a Hugging
// Face file URL or by the downloader's compact "[type:]repo:quantization" key.
//
// The id is not an input: the downloader derives it from the file that arrives, using the
// same rule the listing does, and reports the paths it wrote as the stream's artifacts.
// Disk space is not pre-checked either, because the size is only known once the URL has
// been resolved against the Hub.
func (m *ModelsIndex) DownloadByURL(ctx context.Context, cli client.APIClient, modelURL, mmprojURL string, plat platform.Platform, publish func(e StreamMessage)) error {
	variables := map[string]string{
		"model_url": modelURL,
		// Fixed rather than taken from the caller: it is the only directory the listing
		// scans for undeclared models, so any other value downloads a model that can
		// never be listed.
		"models_repository": llamacppRepository,
	}
	if mmprojURL != "" {
		variables["model_mmproj_url"] = mmprojURL
	}

	return m.Download(ctx, cli, AIModel{
		Deployment: &ModelDeployment{
			Handler: hfHandlerID,
			Variables: []map[string]PlatformDeploymentConfig{
				{plat.BoardName: {Variables: variables}},
			},
		},
	}, plat, publish)
}

// DeclaredByID returns the models-list.yaml entry for id, with no handler run.
//
// It says nothing about whether the model is installed - only what the declaration holds.
// That is enough for a caller that has just installed the model itself and needs the
// name, description and bricks the declaration gives it.
func (m *ModelsIndex) DeclaredByID(id string) (*AIModel, bool) {
	models := m.loadDryModels()
	if i := slices.IndexFunc(models, func(v AIModel) bool { return v.ID == id }); i != -1 {
		return &models[i], true
	}
	return nil, false
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
