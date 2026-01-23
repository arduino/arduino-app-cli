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

	"github.com/arduino/arduino-app-cli/internal/edgeimpulse"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex/aimodel"
)

type AIModelsListResult struct {
	Models []AIModelItem `json:"models"`
}

type AIModelItem struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	ModuleDescription  string            `json:"description"`
	Runner             string            `json:"runner"`
	Bricks             []string          `json:"brick_ids"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	ModelConfiguration map[string]string `json:"model_configuration,omitempty"`
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
			ID:                 model.ID,
			Name:               model.Name,
			ModuleDescription:  model.ModuleDescription,
			Runner:             model.Runner,
			Bricks:             model.Bricks,
			Metadata:           model.Metadata,
			ModelConfiguration: model.ModelConfiguration,
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
		ID:                 model.ID,
		Name:               model.Name,
		ModuleDescription:  model.ModuleDescription,
		Runner:             model.Runner,
		Bricks:             model.Bricks,
		Metadata:           model.Metadata,
		ModelConfiguration: model.ModelConfiguration,
	}, true
}

func InstallEIModel(ctx context.Context, bricksIndex *bricksindex.BricksIndex, eiClient *edgeimpulse.EIClient, modelsDir *paths.Path, projectID int, impulseID int, modelType string, engine string) error {
	project, err := eiClient.GetProjectInfo(ctx, projectID, impulseID)
	if err != nil {
		return err
	}
	id := fmt.Sprintf("ei-model-%d-%d", projectID, impulseID)

	edgeModelsDir := modelsDir.Join(id)
	blobModelsDir := edgeModelsDir.Join("model.eim")

	descr := aimodel.ModelDescriptor{
		ID:          id,
		Name:        project.Name,
		Description: project.Description,
		Metadata: map[string]any{
			"source":        "edgeimpulse",
			"ei-project-id": projectID,
			"ei-impulse-id": impulseID,
			"ei-model-type": modelType,
			"ei-engine":     engine,
		},
		Bricks: buildBrickConfigForEIModel(bricksIndex, project.Category, edgeModelsDir, blobModelsDir),
	}

	err = aimodel.Write(edgeModelsDir, descr)
	if err != nil {
		slog.Error("failed to write EI model metadata", "err", err)
		return err
	}

	modelTypeParam := edgeimpulse.ModelTypeParameter(modelType)
	engineParam := edgeimpulse.ModelEngineParameter(engine)

	version, err := eiClient.GetDeployment(ctx, projectID, modelTypeParam, engineParam)
	if err != nil {
		return err
	}
	if version == nil {
		jobId, err := eiClient.Build(ctx, projectID, modelTypeParam, engineParam)
		if err != nil {
			return err
		}
		err = eiClient.WaitForBuildCompletion(ctx, projectID, *jobId)
		if err != nil {
			return err
		}
	}

	err = eiClient.DownloadAndInstallModel(ctx, blobModelsDir, projectID, impulseID, modelTypeParam, engineParam)
	if err != nil {
		return err
	}
	return nil
}

var mapCategoryToBricks = map[edgeimpulse.ProjectCategory][]string{
	edgeimpulse.ProjectCategoryOther:           {},
	edgeimpulse.ProjectCategoryObjectDetection: {"arduino:object_detection", "arduino:video_object_detection"},
	edgeimpulse.ProjectCategoryImages:          {"arduino:image_classification", "arduino:video_image_classification"},
	edgeimpulse.ProjectCategoryAudio:           {"arduino:audio_classification"},
	edgeimpulse.ProjectCategoryKeywordSpotting: {"arduino:audio_classification", "arduino:keyword_spotting"},
	edgeimpulse.ProjectCategoryAccelerometer:   {"arduino:gesture_recognition", "arduino:anomaly_detection"},
}

func buildBrickConfigForEIModel(bricksIndex *bricksindex.BricksIndex, category *edgeimpulse.ProjectCategory, edgeModelsDir *paths.Path, blobModelsDir *paths.Path) []aimodel.BrickConfig {
	if category == nil {
		return []aimodel.BrickConfig{}
	}
	bricksIds := mapCategoryToBricks[*category]

	var bricksConfig []aimodel.BrickConfig
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

		bricksConfig = append(bricksConfig, aimodel.BrickConfig{
			ID:                 brick.ID,
			ModelConfiguration: modelConfigPerBrick,
		})
	}
	return bricksConfig
}
