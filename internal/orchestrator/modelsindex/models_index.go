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

package modelsindex

import (
	"slices"

	"github.com/arduino/go-paths-helper"
	"github.com/goccy/go-yaml"
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

type AIModel struct {
	ID                 string            `yaml:"-"`
	Name               string            `yaml:"name"`
	ModuleDescription  string            `yaml:"description"`
	Runner             string            `yaml:"runner"`
	Bricks             []string          `yaml:"bricks,omitempty"`
	ModelLabels        []string          `yaml:"model_labels,omitempty"`
	Metadata           map[string]string `yaml:"metadata,omitempty"`
	ModelConfiguration map[string]string `yaml:"model_configuration,omitempty"`
}

type ModelsIndex struct {
	models []AIModel
}

func (m *ModelsIndex) GetModels() []AIModel {
	return m.models
}

func (m *ModelsIndex) GetModelByID(id string) (*AIModel, bool) {
	idx := slices.IndexFunc(m.models, func(v AIModel) bool { return v.ID == id })
	if idx == -1 {
		return nil, false
	}
	return &m.models[idx], true
}

func (m *ModelsIndex) GetModelsByBrick(brick string) []AIModel {
	var matches []AIModel
	for i := range m.models {
		if len(m.models[i].Bricks) > 0 && slices.Contains(m.models[i].Bricks, brick) {
			matches = append(matches, m.models[i])
		}
	}
	if len(matches) == 0 {
		return nil
	}
	return matches
}

func (m *ModelsIndex) GetModelsByBricks(bricks []string) []AIModel {
	var matchingModels []AIModel
	for _, model := range m.models {
		for _, modelBrick := range model.Bricks {
			if slices.Contains(bricks, modelBrick) {
				matchingModels = append(matchingModels, model)
				break
			}
		}
	}
	return matchingModels
}

func GenerateModelsIndexFromFile(dir *paths.Path) (*ModelsIndex, error) {
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
			models[i] = model
		}
	}
	return &ModelsIndex{models: models}, nil
}
