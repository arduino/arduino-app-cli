package orchestrator

import (
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
)

type AIModelsListResult struct {
	Models []AIModelItem `json:"models"`
}

type AIModelItem struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	ModuleDescription  string            `json:"description"`
	Runner             string            `json:"runner"`
	Brick              string            `json:"brick_id"`
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
			Brick:              model.Brick,
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
		Brick:              model.Brick,
		Metadata:           model.Metadata,
		ModelConfiguration: model.ModelConfiguration,
	}, true
}
