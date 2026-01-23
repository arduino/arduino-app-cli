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

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/edgeimpulse"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex/aimodel"
)

// Brick variable names (centralized to avoid typos and ease maintenance)
const (
	VarCustomModelPath            = "CUSTOM_MODEL_PATH"
	VarEIObjDetectionModel        = "EI_OBJ_DETECTION_MODEL"
	VarEIAudioClassificationModel = "EI_AUDIO_CLASSIFICATION_MODEL"
	VarEIClassificationModel      = "EI_CLASSIFICATION_MODEL"
	VarEIKeywordSpottingModel     = "EI_KEYWORD_SPOTTING_MODEL"
	VarEIMotionDetectionModel     = "EI_MOTION_DETECTION_MODEL"
	VarEIVAnomalyDetectionModel   = "EI_V_ANOMALY_DETECTION_MODEL"
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
	// TODO: if it already exit, remove and create again
	id := fmt.Sprintf("ei-model-%d-%d", projectID, impulseID)

	edgeModelsDir := modelsDir.Join(id)
	//TODO: model file name is in two different position.
	blobModelsDir := edgeModelsDir.Join("model.eim")

	descr := aimodel.ModelDescriptor{
		ID:          id,
		Name:        project.Name,
		Description: project.Description,
		Metadata:    buildMedataForEIModel(projectID, impulseID),
	}
	bricksConfig := make([]aimodel.BrickConfig, 0)
	if project.Category != nil {
		bricks := categoryToBricks(project.Category)

		for _, b := range bricks {
			brick, ok := bricksIndex.FindBrickByID(b)
			if !ok {
				slog.Warn("cannot load brick", "id", b)
				continue
			}
			modelConfigPerBrick := map[string]any{}
			for _, variable := range brick.Variables {
				switch variable.Name {
				case VarCustomModelPath:
					modelConfigPerBrick[variable.Name] = edgeModelsDir.String()
				case VarEIObjDetectionModel, VarEIAudioClassificationModel, VarEIClassificationModel, VarEIKeywordSpottingModel, VarEIMotionDetectionModel, VarEIVAnomalyDetectionModel:
					modelConfigPerBrick[variable.Name] = blobModelsDir.String()
				default:
					slog.Warn("variable not found in the bricks config")
				}
			}

			bricksConfig = append(bricksConfig, aimodel.BrickConfig{
				ID:                 brick.ID,
				ModelConfiguration: modelConfigPerBrick,
			})
		}
	}

	descr.Bricks = bricksConfig

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

	// TODO: receive the writer
	err = eiClient.DownloadAndInstallModel(ctx, edgeModelsDir, projectID, impulseID, modelTypeParam, engineParam)
	if err != nil {
		return err
	}

	return nil
}

func categoryToBricks(category *edgeimpulse.ProjectCategory) []string {
	if category == nil {
		return []string{}
	}
	switch *category {
	case edgeimpulse.ProjectCategoryOther:
		return []string{}
	case edgeimpulse.ProjectCategoryObjectDetection:
		return []string{"arduino:object_detection", "arduino:video_object_detection"}
	case edgeimpulse.ProjectCategoryImages:
		return []string{"arduino:image_classification", "arduino:video_image_classification"}
	case edgeimpulse.ProjectCategoryAudio:
		return []string{"arduino:audio_classification"}
	case edgeimpulse.ProjectCategoryKeywordSpotting:
		return []string{"arduino:audio_classification", "arduino:keyword_spotting"}
	case edgeimpulse.ProjectCategoryAccelerometer:
		return []string{"arduino:gesture_recognition", "arduino:anomaly_detection"}
	default:
		return []string{}
	}
}

func buildMedataForEIModel(projectID int, impulseID int) map[string]any {
	return map[string]any{
		"source":        "edgeimpulse",
		"ei-project-id": fmt.Sprintf("%d", projectID),
		"ei-impulse-id": fmt.Sprintf("%d", impulseID),
	}
}
