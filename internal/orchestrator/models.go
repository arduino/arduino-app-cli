// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/arduino/go-paths-helper"

	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"github.com/arduino/arduino-app-cli/internal/api/edgeimpulse"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"

	"github.com/docker/cli/cli/command"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex/custommodel"
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
}

type AIModelsListRequest struct {
	FilterByBrickID []string
}

func AIModelsList(req AIModelsListRequest, modelsIndex *modelsindex.ModelsIndex) AIModelsListResult {
	var collection []modelsindex.AIModel
	if len(req.FilterByBrickID) == 0 {
		collection = modelsIndex.GetModels()
	} else {
		collection = modelsIndex.GetModelsByBricks(req.FilterByBrickID)
	}
	res := AIModelsListResult{Models: make([]AIModelItem, len(collection))}
	for i, model := range collection {
		res.Models[i] = AIModelItem{
			ID:                model.ID,
			Name:              model.Name,
			ModuleDescription: model.ModuleDescription,
			Runner:            model.Runner,
			Bricks:            f.Map(model.Bricks, func(b modelsindex.BrickConfig) string { return b.ID }),
			Metadata:          model.Metadata,
			IsBuiltin:         model.IsInternal,
		}
	}
	return res
}

func AIModelDetails(modelsIndex *modelsindex.ModelsIndex, id string) (AIModelItem, bool) {
	model, found := modelsIndex.GetModelByID(id)
	if !found {
		return AIModelItem{}, false
	}
	return AIModelItem{
		ID:                model.ID,
		Name:              model.Name,
		ModuleDescription: model.ModuleDescription,
		Runner:            model.Runner,
		Bricks:            f.Map(model.Bricks, func(b modelsindex.BrickConfig) string { return b.ID }),
		Metadata:          model.Metadata,
		IsBuiltin:         model.IsInternal,
	}, true
}

var (
	ErrNotFound          = errors.New("model not found")
	ErrConflict          = errors.New("can't delete the model")
	ErrCannotRemoveModel = errors.New("cannot remove an internal model")
)

func AIModelDelete(ctx context.Context, dockerClient command.Cli, cfg config.Configuration, modelsIndex *modelsindex.ModelsIndex, id string, idProvider *app.IDProvider, force bool) (err error) {
	res, found := modelsIndex.GetModelByID(id)
	if !found {
		return fmt.Errorf("%q: %w", id, ErrNotFound)
	}

	if res.IsInternal {
		return ErrCannotRemoveModel
	}

	references, runningAppReference, err := checkForModelReferences(ctx, dockerClient, cfg, idProvider, id)
	if err != nil {
		return err
	}

	hasReferences := len(references) > 0
	isRunning := runningAppReference != nil

	if hasReferences || isRunning {
		if !force {
			return fmt.Errorf("%s: %w", buildModelInUseMessage(references, runningAppReference), ErrConflict)
		}
	}

	if runningAppReference != nil {
		StopApp(ctx, dockerClient, *runningAppReference)
	}

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
	sb.WriteString("The model is")

	if len(references) > 0 {
		sb.WriteString(" referenced by bricks belonging to the following apps: ")
		sb.WriteString(strings.Join(references, ", "))
	}

	if runningAppRef != nil {
		sb.WriteString(" in use by the app ")
		sb.WriteString(runningAppRef.Name)
	}

	return sb.String()
}

// Validate if the model is currently in use or referenced.
// Both checks are performed simultaneously to support the "force" flag logic.
// This allows the user to see both issues before deciding to use the flag
// preventing the second error from being masked.
func checkForModelReferences(ctx context.Context, dockerClient command.Cli, cfg config.Configuration, idProvider *app.IDProvider, modelId string) ([]string, *app.ArduinoApp, error) {
	apps, err := ListApps(ctx, dockerClient, ListAppRequest{
		ShowExamples:                   true,
		ShowApps:                       true,
		IncludeNonStandardLocationApps: true,
	},
		idProvider, cfg)

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

func InstallEIModel(ctx context.Context, bricksIndex *bricksindex.BricksIndex, eiClient *edgeimpulse.EIClient, modelsDir *paths.Path, projectID int, impulseID int, modelType string, engine string, deviceType string) (*custommodel.AiModel, error) {

	project, err := eiClient.GetProjectInfo(ctx, projectID, impulseID)
	if err != nil {
		return nil, err
	}
	id := fmt.Sprintf("ei-model-%d-%d", projectID, impulseID)

	edgeModelsDir := modelsDir.Join(id)
	blobModelsDir := edgeModelsDir.Join("model.eim")

	modelRC, err := eiClient.DownloadAndInstallModel(ctx, blobModelsDir, projectID, impulseID, mType, mEngine, deviceType)
	if err != nil {
		return nil, err
	}

	customModelDescriptor := custommodel.ModelDescriptor{
		ID:          id,
		Name:        project.Name,
		Description: project.Description,
		Metadata: map[string]string{
			"source":        "edgeimpulse",
			"ei-project-id": fmt.Sprintf("%d", projectID),
			"ei-impulse-id": fmt.Sprintf("%d", impulseID),
			"ei-model-type": string(mType),
			"ei-engine":     string(mEngine),
		},
		Bricks: buildBrickConfigForEIModel(bricksIndex, project.Category, edgeModelsDir, blobModelsDir),
	}

	aimodel, err := custommodel.Store(edgeModelsDir, customModelDescriptor, modelRC, "model.eim")
	if err != nil {
		return nil, err
	}

	return &aimodel, nil
}

var mapCategoryToBricks = map[edgeimpulse.ProjectCategory][]string{
	edgeimpulse.ProjectCategoryOther:           {},
	edgeimpulse.ProjectCategoryObjectDetection: {"arduino:object_detection", "arduino:video_object_detection"},
	edgeimpulse.ProjectCategoryImages:          {"arduino:image_classification", "arduino:video_image_classification"},
	edgeimpulse.ProjectCategoryAudio:           {"arduino:audio_classification"},
	edgeimpulse.ProjectCategoryKeywordSpotting: {"arduino:audio_classification", "arduino:keyword_spotting"},
	edgeimpulse.ProjectCategoryAccelerometer:   {"arduino:gesture_recognition", "arduino:anomaly_detection"},
}

func buildBrickConfigForEIModel(bricksIndex *bricksindex.BricksIndex, category *edgeimpulse.ProjectCategory, edgeModelsDir *paths.Path, blobModelsDir *paths.Path) []custommodel.BrickConfig {
	if category == nil {
		return []custommodel.BrickConfig{}
	}
	bricksIds := mapCategoryToBricks[*category]

	bricksConfig := make([]custommodel.BrickConfig, 0)
	for _, b := range bricksIds {
		brick, ok := bricksIndex.FindBrickByID(b)
		if !ok {
			slog.Warn("cannot load brick", "id", b, "category", category)
			continue
		}
		modelConfigPerBrick := map[string]any{}
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
	return bricksConfig
}
