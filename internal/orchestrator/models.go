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
