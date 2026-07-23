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
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/api/edgeimpulse"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
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
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Runner      string                  `json:"runner"`
	Bricks      []string                `json:"brick_ids"`
	Metadata    map[string]string       `json:"metadata,omitempty"`
	IsBuiltIn   bool                    `json:"is_builtin"`
	Size        *uint64                 `json:"size,omitempty"`
	Status      modelsindex.ModelStatus `json:"status"`
}

type AIModelsListRequest struct {
	FilterByBrickID []string
}

func AIModelsList(ctx context.Context, req AIModelsListRequest, modelsIndex *modelsindex.ModelsIndex) AIModelsListResult {
	collection := modelsIndex.GetModels(ctx)
	if len(req.FilterByBrickID) != 0 {
		collection = slices.DeleteFunc(collection, func(model modelsindex.AIModel) bool {
			return !slices.ContainsFunc(model.Bricks, func(brick modelsindex.BrickConfig) bool {
				return slices.Contains(req.FilterByBrickID, brick.ID)
			})
		})
	}

	items := f.Map(collection, func(model modelsindex.AIModel) AIModelItem {
		return AIModelItem{
			ID:          model.ID,
			Name:        model.Name,
			Description: model.Description,
			Runner:      model.Runner,
			Bricks:      f.Map(model.Bricks, func(b modelsindex.BrickConfig) string { return b.ID }),
			Metadata:    model.Metadata,
			IsBuiltIn:   model.IsBuiltIn,
			Status:      model.Status,
			Size: func() *uint64 {
				if model.Size > 0 {
					return &model.Size
				}
				return nil
			}(),
		}
	})
	return AIModelsListResult{Models: items}
}

func AIModelDetails(ctx context.Context, modelsIndex *modelsindex.ModelsIndex, id string) (AIModelItem, bool, error) {

	model, err := modelsIndex.GetModelByID(ctx, id)
	if err != nil {
		return AIModelItem{}, false, err
	}
	if model == nil {
		return AIModelItem{}, false, nil
	}

	return AIModelItem{
		ID:          model.ID,
		Name:        model.Name,
		Description: model.Description,
		Runner:      model.Runner,
		Bricks:      f.Map(model.Bricks, func(b modelsindex.BrickConfig) string { return b.ID }),
		Metadata:    model.Metadata,
		IsBuiltIn:   model.IsBuiltIn,
		Size:        &model.Size,
		Status:      model.Status,
	}, true, nil
}

var (
	ErrNotFound            = errors.New("model not found")
	ErrConflict            = errors.New("can't delete the model")
	ErrCannotRemoveModel   = errors.New("cannot remove a built-in model")
	ErrInsufficientStorage = errors.New("insufficient storage to install the model")
	ErrIncompleteImpulse   = errors.New("impulse not ready for deployment")
)

func AIModelDelete(ctx context.Context, dockerClient command.Cli, cfg config.Configuration, modelsIndex *modelsindex.ModelsIndex, bricksIndex *bricksindex.BricksIndex, platform platform.Platform, id string, idProvider *appid.Provider, force bool) (err error) {
	res, err := modelsIndex.GetModelByID(ctx, id)
	if err != nil {
		return err
	}
	if res == nil {
		return fmt.Errorf("%q: %w", id, ErrNotFound)
	}

	if res.IsBuiltIn {
		return ErrCannotRemoveModel
	}

	references, runningAppReference, err := checkForModelReferences(ctx, dockerClient, cfg, idProvider, bricksIndex, id, platform)
	if err != nil {
		return err
	}

	if len(references) > 0 || runningAppReference != nil {
		if !force {
			return fmt.Errorf("%w. %s", ErrConflict, buildModelInUseMessage(references, runningAppReference))
		}
	}

	if runningAppReference != nil {
		// TODO: we should destroy the app
		if err := StopApp(ctx, dockerClient, platform, *runningAppReference, func(StreamMessage) {}); err != nil {
			slog.Warn("Error while stopping the app using the model", "app", runningAppReference.Name, "error", err.Error())
		}
	}

	err = modelsIndex.Delete(ctx, dockerClient, platform, *res)
	if err != nil {
		return fmt.Errorf("error deleting model %q: %w", id, err)
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
func checkForModelReferences(ctx context.Context, dockerClient command.Cli,
	cfg config.Configuration, idProvider *appid.Provider, bricksIndex *bricksindex.BricksIndex,
	modelId string, platform platform.Platform) ([]string, *app.ArduinoApp, error) {
	apps, err := ListApps(
		ctx, dockerClient, ListAppRequest{
			ShowExamples:                   true,
			ShowApps:                       true,
			IncludeNonStandardLocationApps: true,
		}, idProvider, bricksIndex, cfg, platform)
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
	model, err := modelsIndex.GetModelByID(ctx, modelId)
	if err != nil {
		return fmt.Errorf("error retrieving model %q: %w", modelId, err)
	}
	if model != nil {
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

func InstallEIModel(ctx context.Context, bricksIndex *bricksindex.BricksIndex, modelsIndex *modelsindex.ModelsIndex, dockerClient command.Cli, eiClient *edgeimpulse.EIClient, modelsDir *paths.Path, platform platform.Platform, projectID int, impulseID int) (AIModelItem, error) {

	eiParams, err := platform.EIDeploymentParams()
	if err != nil {
		return AIModelItem{}, err
	}

	id := fmt.Sprintf("ei-model-%d-%d", projectID, impulseID)
	err = isModelInUse(ctx, modelsIndex, dockerClient, id)
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
	// check if there is a deployment and is valid for arduino uno Q or ventuno target, otherwise build it.
	var mversion int
	if len(dpList) == 0 || dpList[0].ImpulseHasChangedSinceDeployment ||
		dpList[0].DeploymentFormat != eiParams.DeviceType || string(dpList[0].Engine) != eiParams.Engine || string(*dpList[0].ModelType) != eiParams.ModelType {

		job, err := eiClient.Build(ctx, projectID, impulseID, eiParams.ModelType, eiParams.Engine, eiParams.DeviceType)
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
			"ei-model-type":         eiParams.ModelType,
			"ei-engine":             eiParams.Engine,
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
		ID:          aimodel.ModelDescriptor.ID,
		Name:        aimodel.ModelDescriptor.Name,
		Description: aimodel.ModelDescriptor.Description,
		Runner:      aimodel.ModelDescriptor.Runner,
		Bricks: f.Map(aimodel.ModelDescriptor.Bricks, func(b custommodel.BrickConfig) string {
			return b.ID
		}),
		Metadata: aimodel.ModelDescriptor.Metadata,
		Status:   modelsindex.InstalledStatus,
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
			if name == "CUSTOM_MODEL_PATH" {
				modelConfigPerBrick[name] = edgeModelsDir.String()
			} else {
				// Leave other variables unset here; they may be user-provided or have defaults
				slog.Debug("skipping non-model variable for EI auto-config", "variable", name, "brick", brick.ID)
			}
		}
		for _, name := range brick.ModelConfigurationVariables {
			// TODO: here we should use the `ai_frameworks_compatibility` for selecting only bricks compatible with Edge Impulse models.
			if strings.HasPrefix(name, "EI_") && strings.HasSuffix(name, "_MODEL") {
				// EI model variables (EI_*_MODEL) get the blob path
				modelConfigPerBrick[name] = blobModelsDir.String()
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
