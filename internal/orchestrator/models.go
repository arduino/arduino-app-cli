// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/arduino/go-paths-helper"
	"github.com/docker/cli/cli/command"
	"github.com/docker/docker/client"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/api/edgeimpulse"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex/custommodel"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

type AIModelsListResult struct {
	Models []AIModelItem `json:"models"`
}

type AIModelItem struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	ModuleDescription string            `json:"description"`
	Runner            string            `json:"runner"`
	Bricks            []string          `json:"brick_ids"`
	Metadata          map[string]string `json:"metadata,omitempty"`
	IsBuiltin         bool              `json:"is_builtin"`
	DiskUsage         *uint64           `json:"disk_usage,omitempty"`
	Installed         bool              `json:"installed"`
}

type AIModelsListRequest struct {
	FilterByBrickID []string
}

func AIModelsList(ctx context.Context, cli client.APIClient, req AIModelsListRequest, modelsIndex *modelsindex.ModelsIndex, cfg config.Configuration, plat platform.Platform) AIModelsListResult {
	var collection []modelsindex.AIModel
	if len(req.FilterByBrickID) == 0 {
		collection = modelsIndex.GetModels(ctx)
	} else {
		collection = modelsIndex.GetModelsByBricks(ctx, req.FilterByBrickID)
	}

	items := f.Map(collection, func(model modelsindex.AIModel) AIModelItem {
		return AIModelItem{
			ID:                model.ID,
			Name:              model.Name,
			ModuleDescription: model.ModuleDescription,
			Runner:            model.Runner,
			Bricks:            f.Map(model.Bricks, func(b modelsindex.BrickConfig) string { return b.ID }),
			Metadata:          model.Metadata,
			IsBuiltin:         model.IsInternal,
			Installed:         model.Installed,
		}
	})
	return AIModelsListResult{Models: items}
}

func AIModelDetails(ctx context.Context, cli client.APIClient, modelsIndex *modelsindex.ModelsIndex, id string, cfg config.Configuration, plat platform.Platform) (AIModelItem, bool) {
	model, found := modelsIndex.GetModelByID(ctx, id)
	if !found {
		return AIModelItem{}, false
	}

	var modelSize *uint64
	if !model.IsInternal && model.ModelFolderPath != nil {
		size, err := getModelSize(model.ModelFolderPath)
		if err != nil {
			slog.Warn(
				"failed to calculate model size",
				"model_id", model.ID,
				"path", model.ModelFolderPath,
				"err", err,
			)
		} else {
			modelSize = &size
		}
	}

	return AIModelItem{
		ID:                model.ID,
		Name:              model.Name,
		ModuleDescription: model.ModuleDescription,
		Runner:            model.Runner,
		Bricks:            f.Map(model.Bricks, func(b modelsindex.BrickConfig) string { return b.ID }),
		Metadata:          model.Metadata,
		IsBuiltin:         model.IsInternal,
		DiskUsage:         modelSize,
		Installed:         model.Installed,
	}, true
}

func getModelSize(dirPath *paths.Path) (uint64, error) {
	if dirPath == nil {
		return 0, fmt.Errorf("directory path is nil")
	}

	files, err := dirPath.ReadDirRecursive()
	if err != nil {
		return 0, err
	}

	var totalSize uint64

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		info, err := file.Stat()
		if err != nil {
			return 0, fmt.Errorf("cannot stat file %s: %w", file.String(), err)
		}

		size := info.Size()
		if size < 0 {
			return 0, fmt.Errorf("file has negative size: %s", file.String())
		}
		totalSize += uint64(size)
	}

	return totalSize, nil
}

var (
	ErrNotFound            = errors.New("model not found")
	ErrConflict            = errors.New("can't delete the model")
	ErrCannotRemoveModel   = errors.New("cannot remove an internal model")
	ErrInsufficientStorage = errors.New("insufficient storage to install the model")
	ErrIncompleteImpulse   = errors.New("impulse not ready for deployment")
)

func AIModelDelete(ctx context.Context, dockerClient command.Cli, cfg config.Configuration, modelsIndex *modelsindex.ModelsIndex, bricksIndex *bricksindex.BricksIndex, platform platform.Platform, id string, idProvider *app.IDProvider, force bool) (err error) {
	res, found := modelsIndex.GetModelByID(ctx, id)
	if !found {
		return fmt.Errorf("%q: %w", id, ErrNotFound)
	}

	if res.IsInternal {
		if res.Deployment == nil || res.Deployment.PreLoaded || res.Deployment.Handler == "" {
			return ErrCannotRemoveModel
		}
	}

	references, runningAppReference, err := checkForModelReferences(ctx, dockerClient, cfg, idProvider, bricksIndex, id)
	if err != nil {
		return err
	}

	if len(references) > 0 || runningAppReference != nil {
		if !force {
			return fmt.Errorf("%w. %s", ErrConflict, buildModelInUseMessage(references, runningAppReference))
		}
	}

	if runningAppReference != nil {
		if err := StopApp(ctx, dockerClient, platform, *runningAppReference, func(StreamMessage) {}); err != nil {
			slog.Warn("Error while stopping the app using the model", "app", runningAppReference.Name, "error", err.Error())
		}
	}

	if res.Deployment != nil && res.Deployment.Handler != "" {
		handler, ok := modelsIndex.Handlers.GetHandlerByID(res.Deployment.Handler)
		if !ok {
			return fmt.Errorf("handler %q not found for model %q", res.Deployment.Handler, id)
		}
		downloadPath := cfg.CustomModelsDir()
		var envVars map[string]string
		if res.Deployment != nil {
			envVars = res.Deployment.VariablesForPlatform(platform.BoardName)
		}
		return runHandlerAction(ctx, dockerClient.Client(), *res, handler.Image, handler.Actions.Delete, handler.Volumes, downloadPath, envVars, nil)
	}

	// Custom model (e.g. Edge Impulse): remove the model folder directly.
	if res.ModelFolderPath == nil {
		slog.Warn("Cannot remove the model with missing model folder", "id", id)
		return nil
	}
	if err := res.ModelFolderPath.RemoveAll(); err != nil {
		return fmt.Errorf("error removing model folder %s", res.ModelFolderPath.String())
	}
	return nil
}

func buildModelInUseMessage(references []string, runningAppRef *app.ArduinoApp) string {
	var sb strings.Builder

	if len(references) > 0 {
		fmt.Fprintf(&sb, "The model is referenced by the following apps: %q.", strings.Join(references, ", "))
	}

	if runningAppRef != nil {
		fmt.Fprintf(&sb, "The model is in use by the app: %q.", runningAppRef.Name)
	}

	return sb.String()
}

// Validate if the model is currently in use or referenced.
// Both checks are performed simultaneously to support the "force" flag logic.
// This allows the user to see both issues before deciding to use the flag
// preventing the second error from being masked.
func checkForModelReferences(ctx context.Context, dockerClient command.Cli, cfg config.Configuration, idProvider *app.IDProvider, bricksIndex *bricksindex.BricksIndex, modelId string) ([]string, *app.ArduinoApp, error) {
	apps, err := ListApps(
		ctx, dockerClient, ListAppRequest{
			ShowExamples:                   true,
			ShowApps:                       true,
			IncludeNonStandardLocationApps: true,
		}, idProvider, bricksIndex, cfg)
	if err != nil {
		return nil, nil, err
	}

	references := make(map[string]struct{})
	var runningAppReference *app.ArduinoApp
	for _, a := range apps.Apps {
		app, err := app.Load(a.ID.ToPath())
		if err != nil {
			slog.Warn("Unable to load app", slog.Any("application name", a.Name))
			continue
		}
		for _, b := range app.Descriptor.Bricks {
			if b.Model == modelId {
				references[app.Name] = struct{}{}
				if a.Status == StatusRunning || a.Status == StatusStarting {
					runningAppReference = &app
				}
			}
		}
	}

	return slices.Collect(maps.Keys(references)), runningAppReference, nil
}

func isModelInUse(ctx context.Context, modelsIndex *modelsindex.ModelsIndex, dockerClient command.Cli, modelId string) error {
	_, found := modelsIndex.FindModelByID(modelId)
	if found {
		runningApp, err := getRunningApp(ctx, dockerClient.Client())
		if err != nil {
			return fmt.Errorf("error retrieving the current running app: %w", err)
		}
		if runningApp != nil {
			app, err := app.Load(runningApp.FullPath)
			if err != nil {
				return fmt.Errorf("error loading app: %w", err)
			}
			for _, b := range app.Descriptor.Bricks {
				if b.Model == modelId {
					return fmt.Errorf("the model is in use by the running app %s, can't be updated", app.Name)
				}
			}
		}
	}
	return nil
}

func InstallEIModel(ctx context.Context, bricksIndex *bricksindex.BricksIndex, modelsIndex *modelsindex.ModelsIndex, dockerClient command.Cli, eiClient *edgeimpulse.EIClient, modelsDir *paths.Path, projectID int, impulseID int) (AIModelItem, error) {

	// TODO these parameters aim to build a model optimized for the Imola hardware, they should change based on the target device
	mType := "float32"
	mEngine := "tflite"
	deviceType := "runner-linux-aarch64"
	var mversion int

	id := fmt.Sprintf("ei-model-%d-%d", projectID, impulseID)
	err := isModelInUse(ctx, modelsIndex, dockerClient, id)
	if err != nil {
		return AIModelItem{}, fmt.Errorf("cannot install EI model: %w", err)
	}

	project, err := eiClient.GetProjectInfo(ctx, projectID, impulseID)
	if err != nil {
		return AIModelItem{}, err
	}

	if !project.ImpulseState.Complete {
		return AIModelItem{}, fmt.Errorf("%w for project %d impulse %d", ErrIncompleteImpulse, projectID, impulseID)
	}

	dpList, err := eiClient.GetDeploymentHistory(ctx, projectID, impulseID, 1)
	if err != nil {
		return AIModelItem{}, err
	}
	// check if there is a deployment and si valid for arduino uno Q, otherwise build it.
	if len(dpList) == 0 || dpList[0].ImpulseHasChangedSinceDeployment ||
		dpList[0].DeploymentFormat != deviceType || string(dpList[0].Engine) != mEngine || string(*dpList[0].ModelType) != mType {

		job, err := eiClient.Build(ctx, projectID, impulseID, mType, mEngine, deviceType)
		if err != nil {
			return AIModelItem{}, err
		}
		err = eiClient.WaitForBuildCompletion(ctx, projectID, job.JobID)
		if err != nil {
			return AIModelItem{}, err
		}
		mversion = job.DeploymentVersion
	} else {
		mversion = dpList[0].DeploymentVersion
	}
	edgeModelsDir := modelsDir.Join("custom-ei").Join(id)
	blobModelsDir := edgeModelsDir.Join("model.eim")

	modelRC, err := eiClient.DownloadHistoricDeployment(ctx, projectID, mversion)
	if err != nil {
		return AIModelItem{}, err
	}

	impulse, err := eiClient.GetImpulseInfo(ctx, projectID, impulseID)
	if err != nil {
		return AIModelItem{}, err
	}

	bricks, err := buildBrickConfigForEIModel(bricksIndex, project.Details.Category, impulse.LearnBlocks, edgeModelsDir, blobModelsDir)
	if err != nil {
		return AIModelItem{}, err
	}
	customModelDescriptor := custommodel.ModelDescriptor{
		ID:          id,
		Runner:      "brick",
		Name:        project.Details.Name,
		Description: project.Details.Name,
		Metadata: map[string]string{
			"source":                "edgeimpulse",
			"ei-project-id":         strconv.Itoa(projectID),
			"ei-impulse-id":         strconv.Itoa(impulseID),
			"ei-impulse-name":       impulse.Name,
			"ei-model-type":         mType,
			"ei-engine":             mEngine,
			"ei-last-modified":      project.Details.LastModified.Local().Format(time.RFC3339Nano),
			"ei-deployment-version": strconv.Itoa(mversion),
		},
		Bricks: bricks,
	}

	aimodel, err := custommodel.Store(edgeModelsDir, customModelDescriptor, modelRC, "model.eim")
	if err != nil {
		if errors.Is(err, syscall.ENOSPC) {
			return AIModelItem{}, ErrInsufficientStorage
		}
		return AIModelItem{}, err
	}

	return AIModelItem{
		ID:                aimodel.ModelDescriptor.ID,
		Name:              aimodel.ModelDescriptor.Name,
		ModuleDescription: aimodel.ModelDescriptor.Description,
		Runner:            aimodel.ModelDescriptor.Runner,
		Bricks: f.Map(aimodel.ModelDescriptor.Bricks, func(b custommodel.BrickConfig) string {
			return b.ID
		}),
		Metadata: aimodel.ModelDescriptor.Metadata,
	}, nil
}

func buildBrickConfigForEIModel(bricksIndex *bricksindex.BricksIndex, category *edgeimpulse.ProjectCategory, impulse []edgeimpulse.ImpulseLearnBlock, edgeModelsDir *paths.Path, blobModelsDir *paths.Path) ([]custommodel.BrickConfig, error) {
	if category == nil {
		return []custommodel.BrickConfig{}, nil
	}

	bricksIds := mapCategoryToBricks(*category, impulse)

	bricksConfig := make([]custommodel.BrickConfig, 0)
	for _, b := range bricksIds {
		brick, ok := bricksIndex.FindBrickByID(b)
		if !ok {
			slog.Warn("cannot load brick", "id", b, "category", category)
			return nil, fmt.Errorf("brick with id %q not found for category %q", b, *category)
		}
		modelConfigPerBrick := make(map[string]string)
		for _, variable := range brick.Variables {
			name := variable.Name
			switch {
			case name == "CUSTOM_MODEL_PATH":
				modelConfigPerBrick[name] = edgeModelsDir.String()
			case strings.HasPrefix(name, "EI_") && strings.HasSuffix(name, "_MODEL"):
				// EI model variables (EI_*_MODEL) get the blob path
				modelConfigPerBrick[name] = blobModelsDir.String()
			default:
				// Leave other variables unset here; they may be user-provided or have defaults
				slog.Debug("skipping non-model variable for EI auto-config", "variable", name, "brick", brick.ID)
			}
		}

		bricksConfig = append(bricksConfig, custommodel.BrickConfig{
			ID:                 brick.ID,
			ModelConfiguration: modelConfigPerBrick,
		})
	}
	return bricksConfig, nil
}

func mapCategoryToBricks(eiCategory edgeimpulse.ProjectCategory, lb []edgeimpulse.ImpulseLearnBlock) []string {
	switch eiCategory {
	case edgeimpulse.ProjectCategoryObjectDetection:
		return []string{"arduino:object_detection", "arduino:video_object_detection"}
	case edgeimpulse.ProjectCategoryImages:
		if slices.ContainsFunc(lb, func(block edgeimpulse.ImpulseLearnBlock) bool {
			return block.Type == edgeimpulse.KerasVisualAnomaly
		}) {
			return []string{"arduino:visual_anomaly_detection"}
		}
		return []string{"arduino:image_classification", "arduino:video_image_classification"}
	case edgeimpulse.ProjectCategoryAudio:
		return []string{"arduino:audio_classification"}
	case edgeimpulse.ProjectCategoryKeywordSpotting:
		return []string{"arduino:audio_classification", "arduino:keyword_spotting"}
	case edgeimpulse.ProjectCategoryAccelerometer:
		return []string{"arduino:motion_detection", "arduino:vibration_anomaly_detection"}
	default:
		return []string{}
	}
}

var (
	ErrModelDownloadFailed = fmt.Errorf("model download failed")
	ErrNoDeploymentHandler = errors.New("model has no deployment handler")
)

func InstallModelByHandler(ctx context.Context, cli client.APIClient, modelID string, modelsIndex *modelsindex.ModelsIndex, cfg config.Configuration, plat platform.Platform, cb func(item StreamMessage)) error {
	model, found := modelsIndex.GetModelByID(ctx, modelID)
	if !found {
		return fmt.Errorf("%q: %w", modelID, ErrNotFound)
	}

	handler, ok := modelsIndex.Handlers.GetHandlerByID(model.Deployment.Handler)
	if !ok {
		return fmt.Errorf("handler %q not found for model %q", model.Deployment.Handler, modelID)
	}

	downloadPath := cfg.CustomModelsDir()
	if err := downloadPath.MkdirAll(); err != nil {
		return fmt.Errorf("cannot create download location %q: %w", downloadPath, err)
	}

	var envVars map[string]string
	if model.Deployment != nil {
		envVars = model.Deployment.VariablesForPlatform(plat.BoardName)
	}

	if model.Installed {
		slog.Info("model already installed", "model", modelID)
		cb(StreamMessage{data: "Model already installed"})
		return nil
	}

	if info, err := fetchModelInfo(ctx, cli, *model, handler, downloadPath, envVars); err != nil {
		slog.Warn("could not fetch model info", "model", modelID, "err", err)
	} else if info != nil && info.sizeBytes > 0 {
		if err := hasSufficientDiskSpace(downloadPath, info.sizeBytes); err != nil {
			return err
		}
	}

	slog.Info("installing model", "model", modelID, "path", downloadPath)
	installResponse := func(e ModelInstallEvent) {
		// TODO: send the progress by capture the STDout of the container as raw string
		cb(StreamMessage{data: e.Description})
	}
	return runHandlerAction(ctx, cli, *model, handler.Image, handler.Actions.Download, handler.Volumes, downloadPath, envVars, installResponse)
}

type modelInfoResult struct {
	sizeBytes int64
}

// fetchModelInfo runs the info action and returns the reported model size.
// Returns nil if the handler has no info action defined.
func fetchModelInfo(ctx context.Context, cli client.APIClient, model modelsindex.AIModel, handler modelsindex.ModelHandler, downloadPath *paths.Path, envVars map[string]string) (*modelInfoResult, error) {
	if len(handler.Actions.Info) == 0 {
		return nil, nil
	}
	var result modelInfoResult
	publish := func(e ModelInstallEvent) {
		if e.Total > 0 {
			result.sizeBytes = e.Total
		}
	}
	if err := runHandlerAction(ctx, cli, model, handler.Image, handler.Actions.Info, handler.Volumes, downloadPath, envVars, publish); err != nil {
		return nil, fmt.Errorf("info action: %w", err)
	}
	return &result, nil
}

// hasSufficientDiskSpace checks whether the filesystem containing path has at
// least requiredBytes of free space. It walks up to the first existing ancestor
// if path does not yet exist.
func hasSufficientDiskSpace(path *paths.Path, requiredBytes int64) error {
	target := path
	for target != nil && target.NotExist() {
		target = target.Parent()
	}
	if target == nil {
		return nil // cannot determine filesystem, skip check
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(target.String(), &stat); err != nil {
		return fmt.Errorf("cannot check disk space: %w", err)
	}
	available := int64(stat.Bavail) * int64(stat.Bsize)
	if available < requiredBytes {
		return ErrInsufficientStorage
	}
	return nil
}
