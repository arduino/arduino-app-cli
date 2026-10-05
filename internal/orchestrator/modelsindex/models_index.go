// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package modelsindex

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/docker/cli/cli/command"
	"github.com/moby/moby/client"
	"github.com/shirou/gopsutil/v4/disk"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex/custommodel"
	"github.com/arduino/arduino-app-cli/internal/platform"

	"github.com/arduino/go-paths-helper"
	"github.com/goccy/go-yaml"
	"go.bug.st/f"
)

type ModelStatus string

type AIModel struct {
	ID              string            `yaml:"-"`
	ModelFolderPath *paths.Path       `yaml:"-"`
	Name            string            `yaml:"name"`
	Description     string            `yaml:"description"`
	Handler         string            `yaml:"handler"`
	Runner          string            `yaml:"runner"`
	Bricks          []BrickConfig     `yaml:"bricks,omitempty"`
	ModelLabels     []string          `yaml:"model_labels,omitempty"`
	Metadata        map[string]string `yaml:"metadata,omitempty"`
	SupportedBoards []string          `yaml:"supported_boards,omitempty"`
	Deployment      *ModelDeployment  `yaml:"deployment,omitempty"`
	Preinstalled    bool              `yaml:"-"` // a model is considered built-in if it is in the models-list.yaml and the "pre-loaded" flag is true
	Origin          ModelOrigin       `yaml:"-"`
	Status          ModelStatus       `yaml:"-"`
	SizeBytes       uint64            `yaml:"-"`
}

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

const (
	InstalledStatus    ModelStatus = "installed"
	NotInstalledStatus ModelStatus = "not-installed"
	// DownloadingStatus is a transfer in progress, or one interrupted before it
	// finished: the handler's ".download" marker is still there.
	DownloadingStatus ModelStatus = "downloading"
)

var ErrEmptyCatalog = errors.New("models listing returned no models: models-list.yaml missing")

func (s ModelStatus) AllowedStatuses() []ModelStatus {
	return []ModelStatus{InstalledStatus, NotInstalledStatus, DownloadingStatus}
}

// ModelOrigin says where a model came from, and so whether the id alone installs it
// again. Derived here, not read from the handler's "model_origin".
type ModelOrigin string

const (
	// CuratedOrigin: declared by models-list.yaml, so the id is the whole install request.
	CuratedOrigin ModelOrigin = "curated"
	// UserOrigin: downloaded from a source the caller supplied, and installing it again
	// needs that source again.
	UserOrigin ModelOrigin = "user"
)

func (o ModelOrigin) AllowedOrigins() []ModelOrigin {
	return []ModelOrigin{CuratedOrigin, UserOrigin}
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
	mu              sync.RWMutex
	cached          []AIModel
	locksDir        *paths.Path
}

// Lookup answers several model queries against at most one listing run. Not safe for
// concurrent use.
type Lookup struct {
	idx    *ModelsIndex
	models []AIModel
	dry    []AIModel
	err    error
	loaded bool
}

func (m *ModelsIndex) Refresh(ctx context.Context) ([]AIModel, error) {

	models, err := m.listModels(ctx)
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, ErrEmptyCatalog
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.cached = models

	return models, nil
}

func (m *ModelsIndex) snapshot() []AIModel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return slices.Clone(m.cached)
}

func (m *ModelsIndex) NewLookup() *Lookup {
	return &Lookup{idx: m}
}

// listing runs on first use only, and remembers a failure: retrying it per query would
// mean a container start per question.
func (l *Lookup) listing(ctx context.Context) error {
	if l.loaded {
		return l.err
	}
	var models []AIModel

	models = l.idx.snapshot()

	if models == nil {
		models, l.err = l.idx.Refresh(ctx)
		l.loaded = true
	}
	if l.err != nil {
		return l.err
	}
	l.models = slices.Clone(models)
	l.loaded = true
	return nil
}

func (l *Lookup) ByID(ctx context.Context, id string) (*AIModel, error) {

	if err := l.listing(ctx); err != nil {
		return nil, fmt.Errorf("cannot determine install status for model %q: %w", id, err)
	}
	idx := slices.IndexFunc(l.models, func(v AIModel) bool { return v.ID == id })
	if idx == -1 {
		return nil, nil
	}
	return &l.models[idx], nil
}

// All answers every model the index knows. A listing that failed leaves the declared ones,
// and says so.
func (l *Lookup) All(ctx context.Context) ([]AIModel, error) {
	err := l.listing(ctx)
	return l.models, err
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

// ModelForBrick resolves a model the brick can use, by its plain id. Nil when no such
// model exists or the brick cannot use it; the model, so a caller stores model.ID.
func (l *Lookup) ModelForBrick(ctx context.Context, modelID, brickID string) (*AIModel, error) {
	model, err := l.ByID(ctx, modelID)
	if err != nil || model == nil {
		return nil, err
	}
	if !slices.ContainsFunc(model.Bricks, func(b BrickConfig) bool { return b.ID == brickID }) {
		return nil, nil
	}
	return model, nil
}

// NeedsNoDownload reports whether the model is there already - built-in, pre-loaded, or
// custom - rather than something a handler has to write to disk.
func (m AIModel) NeedsNoDownload() bool {
	return m.Deployment == nil || m.Deployment.PreLoaded || m.Deployment.Handler == ""
}

func (m *ModelsIndex) listModels(ctx context.Context) ([]AIModel, error) {
	known := m.loadDryModels()
	if m.Handlers == nil || m.cli == nil {
		return known, nil
	}
	models, err := m.Handlers.getModelsInfo(ctx, m.cli, known)
	if err != nil {
		return nil, err
	}
	return models, nil
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
		locksDir:        cfg.ModelLocksDir(),
	}, nil
}

// WriteModelsList writes the models into dir as models-list.yaml states them.
func WriteModelsList(dir *paths.Path, models []AIModel) error {
	list := assetsModelList{Models: make([]map[string]AIModel, 0, len(models))}
	for _, model := range models {
		list.Models = append(list.Models, map[string]AIModel{model.ID: model})
	}
	data, err := yaml.Marshal(list)
	if err != nil {
		return err
	}
	return dir.Join(modelsListFileName).WriteFile(data)
}

const modelsListFileName = "models-list.yaml"

func loadInternalModels(dir *paths.Path, handlers *HandlersIndex) ([]AIModel, error) {
	if dir == nil {
		// skip loading internal models
		return []AIModel{}, nil
	}

	content, err := dir.Join(modelsListFileName).ReadFile()
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
			model.Origin = CuratedOrigin
			model.Status = NotInstalledStatus

			if sizeMBStr, ok := model.Metadata["model_size_mb"]; ok {
				if sizeMB, err := strconv.ParseFloat(sizeMBStr, 64); err == nil && sizeMB > 0 {
					model.SizeBytes = uint64(sizeMB * 1024 * 1024)
				}
			}

			if model.Deployment == nil {
				model.Preinstalled = true
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
					model.Preinstalled = true
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

		var sizeBytes uint64
		if modelFileInfo, err := m.FullPath.Join("model.eim").Stat(); err != nil {
			slog.Warn("unable to stat custom model file", slog.String("error", err.Error()), "path", m.FullPath.Join("model.eim"))
		} else if size := modelFileInfo.Size(); size > 0 {
			sizeBytes = uint64(size)
		}

		models = append(models, AIModel{
			ID:          m.ModelDescriptor.ID,
			Name:        m.ModelDescriptor.Name,
			Description: m.ModelDescriptor.Description,
			Handler:     "ei-handler",
			Bricks: f.Map(m.ModelDescriptor.Bricks, func(b custommodel.BrickConfig) BrickConfig {
				return BrickConfig(b)
			}),
			Metadata:        m.ModelDescriptor.Metadata,
			ModelFolderPath: m.FullPath,
			Preinstalled:    false,
			Origin:          UserOrigin,
			Status:          InstalledStatus,
			SizeBytes:       sizeBytes,
		})
	}

	return models, nil
}

// IsKnown reports whether the index holds id in its own files, with no handler run. It is
// the question the install route answers before its stream opens.
func (m *ModelsIndex) IsKnown(id string) bool {
	_, found := m.known(id)
	return found
}

// known returns what the index's own files say about id, with no handler run: a
// models-list.yaml entry, or a custom model. It is not a state - the listing owns the
// install status, the size on disk and the record's metadata - so it stays in this package.
func (m *ModelsIndex) known(id string) (*AIModel, bool) {
	models := m.loadDryModels()
	if i := slices.IndexFunc(models, func(v AIModel) bool { return v.ID == id }); i != -1 {
		return &models[i], true
	}
	return nil, false
}

type PrecheckResult struct {
	SizeBytes uint64
	Installed bool
}

// PrecheckInstall runs, before a curated install opens its stream, everything that can
// refuse it: the lock, the hub, what is on disk, the free space. Hold unlock until the
// install returns.
func (m *ModelsIndex) PrecheckInstall(ctx context.Context, docker command.Cli, id string, plat platform.Platform) (func(), PrecheckResult, error) {
	model, found := m.known(id)
	if !found {
		return func() {}, PrecheckResult{}, fmt.Errorf("no model with id %q: %w", id, ErrUnknownModel)
	}
	if model.NeedsNoDownload() {
		// Preinstalled: nothing to fetch, nothing to lock.
		return func() {}, PrecheckResult{Installed: true}, nil
	}
	// The docker client is read only now: a caller with nothing to download passes none.
	return m.precheck(ctx, docker.Client(), *model, id, false, plat)
}

// PrecheckDownload is PrecheckInstall for a Hugging Face download by link. Files on disk
// are not enough to call it installed: a download stopped after its file landed but
// before its record was written leaves a file the listing ignores and the API cannot
// delete. Only a listed model recorded from this link counts; anything else downloads
// again, and the downloader clears the leftover first.
func (m *ModelsIndex) PrecheckDownload(ctx context.Context, cli client.APIClient, modelURL, mmprojURL string, plat platform.Platform) (func(), PrecheckResult, error) {
	unlock, res, err := m.precheck(ctx, cli, userHFModel(modelURL, mmprojURL, plat), modelURL, true, plat)
	if err != nil || !res.Installed {
		return unlock, res, err
	}
	recorded, err := m.listedFromSource(ctx, modelURL)
	if err != nil {
		unlock()
		return func() {}, PrecheckResult{}, err
	}
	if !recorded {
		slog.Warn("model file found without a download record, downloading again", "url", modelURL)
		res.Installed = false
	}
	return unlock, res, nil
}

// listedFromSource reports whether the listing holds a model recorded from url.
func (m *ModelsIndex) listedFromSource(ctx context.Context, url string) (bool, error) {
	models, err := m.NewLookup().All(ctx)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(models, func(v AIModel) bool {
		return v.Metadata["source-model-url"] == url
	}), nil
}

func (m *ModelsIndex) precheck(ctx context.Context, cli client.APIClient, model AIModel, key string, isUser bool, plat platform.Platform) (func(), PrecheckResult, error) {
	var res PrecheckResult

	// 1. lock
	unlock, err := lockModel(m.locksDir, key)
	if err != nil {
		return func() {}, res, err
	}
	fail := func(err error) (func(), PrecheckResult, error) {
		unlock()
		return func() {}, PrecheckResult{}, err
	}

	handler, ok := m.Handlers.GetHandlerByID(model.Deployment.Handler)
	if !ok {
		return fail(fmt.Errorf("handler %q not found for model %q", model.Deployment.Handler, model.ID))
	}
	vars := model.Deployment.VariablesForPlatform(plat.BoardName)

	// 2. info → size, or why the model cannot be fetched
	var infoErr bool
	var infoErrText string
	err = m.Handlers.runAction(ctx, cli, handler, ActionInfo, vars, func(line string) {
		size, text, isErr := parseInfoLine(line)
		if isErr {
			infoErr, infoErrText = true, text
			return
		}
		if size > 0 {
			res.SizeBytes = size
		}
	})
	switch {
	case errors.Is(err, ErrNoAction):
		// no info action: size unknown, carry on
	case infoErr:
		if isUser {
			return fail(classifyInfoError(infoErrText))
		}
		return fail(fmt.Errorf("%w: %s", ErrInfoFailed, infoErrText))
	case err != nil:
		// the container failed without saying why (crash, missing image)
		return fail(fmt.Errorf("%w: %w", ErrInfoFailed, err))
	}

	_ = m.Handlers.runAction(ctx, cli, handler, ActionCheck, vars, func(line string) {
		if installed, ok := checkLineInstalled(line); ok {
			res.Installed = installed
		}
	})

	// 4. disk
	if res.SizeBytes > 0 && !res.Installed {
		if err := hasSufficientDiskSpace(m.modelsDir, res.SizeBytes); err != nil {
			return fail(fmt.Errorf("%w: %w", ErrInsufficientStorage, err))
		}
	}

	return unlock, res, nil
}

var (
	ErrInfoFailed       = errors.New("cannot inspect the model")          // curated: always this → 500
	ErrBadModelURL      = errors.New("invalid model URL")                 // 400
	ErrModelNotFound    = errors.New("model not found on the hub")        // 404
	ErrModelForbidden   = errors.New("model is private or gated")         // 403
	ErrModelGone        = errors.New("model repository disabled")         // 410
	ErrUnsupportedModel = errors.New("model not supported on this board") // 422
	ErrHubUnreachable   = errors.New("cannot reach the model hub")        // 502
)

// Install fetches the model id names in the internal model list and answers with it as
// installed. The declaration describes it; only the size comes from what landed.
//
// A model installed by its declaration is returned as it is: there is nothing to fetch.
func (m *ModelsIndex) Install(ctx context.Context, dockerClient command.Cli, id string, plat platform.Platform, publish func(e StreamMessage)) (AIModel, error) {

	model, found := m.known(id)
	if !found {
		return AIModel{}, fmt.Errorf("no model with id %q: %w", id, ErrUnknownModel)
	}
	if model.NeedsNoDownload() {
		// It is there already, so the docker client is not read: a caller with nothing to
		// download passes none.
		return *model, nil
	}

	downloaded, err := m.runDownload(ctx, dockerClient.Client(), *model, plat, publish)
	if err != nil {
		return AIModel{}, err
	}

	models, err := m.Refresh(context.WithoutCancel(ctx))
	if err != nil {
		return AIModel{}, fmt.Errorf("model %q downloaded, but the listing failed: %w", downloaded.ID, err)
	}

	return models[slices.IndexFunc(models, func(v AIModel) bool { return v.ID == model.ID })], nil
}

// DownloadByURL fetches a model no models-list.yaml entry declares, named by a Hugging
// Face file URL.
//
// The id is not an input: the downloader makes it from the file that arrives, with the same
// rule as the listing, and reports it on the stream. The id contains the repository
// directory, so two owners with the same file name stay two models. The disk space check is
// PrecheckDownload's: call it first, and hold its lock until this returns.
func (m *ModelsIndex) DownloadByURL(ctx context.Context, cli client.APIClient, modelURL, mmprojURL string, plat platform.Platform, publish func(e StreamMessage)) (AIModel, error) {

	downloaded, err := m.runDownload(ctx, cli, userHFModel(modelURL, mmprojURL, plat), plat, publish)
	if err != nil {
		return AIModel{}, err
	}
	if downloaded == nil {
		return AIModel{}, ErrNoModelReported
	}

	models, err := m.Refresh(context.WithoutCancel(ctx))
	if err != nil {
		return AIModel{}, fmt.Errorf("model %q downloaded, but the listing failed: %w", downloaded.ID, err)
	}
	i := slices.IndexFunc(models, func(v AIModel) bool { return v.ID == downloaded.ID })
	if i == -1 {
		return AIModel{}, fmt.Errorf("model %q was downloaded but is not listed: %w", downloaded.ID, ErrNotListed)
	}
	return models[i], nil
}

// runDownload runs one handler's download action and keeps the model its stream names. An
// error event ends the run as ErrDownloadReported: publish has already carried it out.
func (m *ModelsIndex) runDownload(ctx context.Context, cli client.APIClient, model AIModel, plat platform.Platform, publish func(e StreamMessage)) (*DownloadedModel, error) {
	if model.NeedsNoDownload() {
		// Guarded here too: the alternative is dereferencing a nil Deployment.
		return nil, fmt.Errorf("model %q has nothing to download: %w", model.ID, ErrNoHandler)
	}
	if m.Handlers == nil {
		return nil, fmt.Errorf("no handlers are configured: %w", ErrNoHandler)
	}
	handler, ok := m.Handlers.GetHandlerByID(model.Deployment.Handler)
	if !ok {
		return nil, fmt.Errorf("handler %q not found for model %q", model.Deployment.Handler, model.ID)
	}

	var downloaded *DownloadedModel
	var reported bool

	lineParser := func(line string) {
		parseDownloadHandlerLine(line, func(e StreamMessage) {
			if named := e.GetModel(); named != nil {
				downloaded = named
			}
			reported = reported || e.GetType() == ErrorType
			publish(e)
		})
	}

	err := m.Handlers.downloadModel(ctx, cli, model, handler, plat, lineParser)

	if ctx.Err() != nil {
		if _, rerr := m.Refresh(context.WithoutCancel(ctx)); rerr != nil {
			slog.Warn("download cancelled; refreshing the models listing failed", "model", model.ID, "err", rerr)
		}
		return nil, ctx.Err()
	}

	if reported {
		return nil, errors.Join(ErrDownloadReported, err) // err may be nil: Join drops it
	}
	if err != nil {
		return nil, err
	}
	return downloaded, nil
}

func (m *ModelsIndex) Delete(ctx context.Context, dockerClient command.Cli, platform platform.Platform, model AIModel) error {
	deleteKey := lockKey(model, platform.BoardName)
	unlock, err := lockModel(m.locksDir, deleteKey)
	if err != nil {
		return err
	}
	defer unlock()

	if model.Deployment != nil && model.Deployment.Handler != "" {
		// Internal model: run the delete action using the handler.
		handler, ok := m.Handlers.GetHandlerByID(model.Deployment.Handler)
		if !ok {
			return fmt.Errorf("handler %q not found for model %q", model.Deployment.Handler, model.ID)
		}
		if err := m.Handlers.deleteInternalModel(ctx, dockerClient.Client(), model, handler, platform); err != nil {
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

	_, err = m.Refresh(context.WithoutCancel(ctx))
	if err != nil {
		return fmt.Errorf("model %q deleted, but the listing failed: %w", model.ID, err)
	}
	return nil
}

var (
	ErrInsufficientStorage = errors.New("insufficient storage to install model")
	ErrNoHandler           = errors.New("no handler to run")
	ErrNoAction            = errors.New("handler does not define this action")
	ErrUnknownModel        = errors.New("model not in the internal model list")
	ErrNoModelReported     = errors.New("download named no model: a newer models-downloader image is required")
	ErrNotListed           = errors.New("model not listed")
	// ErrDownloadReported ends a download whose handler reported an error event, which
	// publish has already carried to the caller.
	ErrDownloadReported = errors.New("the download reported an error")
)

func hasSufficientDiskSpace(path *paths.Path, requiredBytes uint64) error {
	diskStats, err := disk.Usage(path.String())
	if err != nil && !errors.Is(err, syscall.ENOENT) {
		return err
	}
	if diskStats != nil {
		// Free is what an unprivileged process can still write (Bavail): Total-Used would
		// also count the blocks reserved for root, which the download container cannot use.
		if requiredBytes > diskStats.Free {
			return fmt.Errorf("%w: model needs %d bytes, %d bytes free", ErrInsufficientStorage, requiredBytes, diskStats.Free)
		}
		return nil
	}
	return nil
}

func userHFModel(modelURL, mmprojURL string, plat platform.Platform) AIModel {
	variables := map[string]string{
		"model_url": modelURL,

		"models_repository": llamacppRepository,
	}
	if mmprojURL != "" {
		variables["model_mmproj_url"] = mmprojURL
	}
	return AIModel{
		Origin: UserOrigin,
		Deployment: &ModelDeployment{
			Handler: hfHandlerID,
			Variables: []map[string]PlatformDeploymentConfig{
				{plat.BoardName: {Variables: variables}},
			},
		},
	}
}

var infoErrorPatterns = []struct {
	contains string
	err      error
}{
	{"Invalid Hugging Face URL:", ErrBadModelURL},
	{"does not exist, or is not public", ErrModelNotFound}, // repo missing, or private seen anonymously
	{"is gated", ErrModelForbidden},
	{"is private", ErrModelForbidden},
	{"has been disabled by its authors", ErrModelGone},
	{"Revision '", ErrModelNotFound},       // "Revision 'x' does not exist in …"
	{"File '", ErrModelNotFound},           // "File 'x' does not exist in …"
	{"No file matching", ErrModelNotFound}, // quantization not in the repo
	{"Not supported quantization", ErrUnsupportedModel},
	{"Could not verify Hugging Face repository", ErrHubUnreachable}, // network, DNS, proxy
}

func classifyInfoError(text string) error {
	for _, p := range infoErrorPatterns {
		if strings.Contains(text, p.contains) {
			return fmt.Errorf("%w: %s", p.err, text)
		}
	}
	return fmt.Errorf("%w: %s", ErrInfoFailed, text)
}

func parseDownloadHandlerLine(line string, publish func(StreamMessage)) {
	e, ok := parseHandlerEvent(line)
	if !ok {
		slog.Debug("non-JSON stdout from handler", "line", line)
		return
	}

	switch e.Event {
	case "start":
		publish(NewInfoMessage(e.Description, nil))
	case "update":
		var pct float32
		if e.Total > 0 {
			pct = float32(e.Current) / float32(e.Total) * 100
		}
		publish(NewProgressMessage(Progress{Name: e.Description, Current: e.Current, Total: e.Total, Progress: pct}))
	case "complete":
		publish(NewDoneMessage("download complete"))
	case "info":
		// Reported only once the handler has recorded the model, so an id here means a
		// later listing can resolve it too.
		var named *DownloadedModel
		if e.ModelID != "" {
			named = &DownloadedModel{ID: e.ModelID, Size: e.Size()}
		}
		publish(NewInfoMessage(e.Description, named))
	case "error":
		publish(NewErrorMessage(e.Description))
	default:
		slog.Warn("unknown event from handler", "event", e.Event, "line", line)
	}
}

func parseInfoLine(line string) (sizeBytes uint64, errText string, isErr bool) {
	e, ok := parseHandlerEvent(line)
	if !ok {
		return 0, "", false
	}
	switch {
	case e.IsError():
		return 0, e.Description, true
	case e.Event == "stat":
		return e.Size(), "", false
	default:
		return 0, "", false
	}
}
