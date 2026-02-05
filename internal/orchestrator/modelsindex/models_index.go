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
	"errors"
	"log/slog"
	"slices"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex/custommodel"

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

type AIModel struct {
	ID                string            `yaml:"-"`
	ModelFolderPath   *paths.Path       `yaml:"-"`
	Name              string            `yaml:"name"`
	ModuleDescription string            `yaml:"description"`
	Runner            string            `yaml:"runner"`
	Bricks            []BrickConfig     `yaml:"bricks,omitempty"`
	ModelLabels       []string          `yaml:"model_labels,omitempty"`
	Metadata          map[string]string `yaml:"metadata,omitempty"`
	IsInternal        bool              `yaml:"-"`
}

type BrickConfig struct {
	ID                 string            `yaml:"id"`
	ModelConfiguration map[string]string `yaml:"model_configuration"`
}

type ModelsIndex struct {
	InternalModels []AIModel
	modelsDir      *paths.Path
}

func (m *ModelsIndex) GetModels() []AIModel {
	return m.loadModels()
}

func (m *ModelsIndex) GetModelByID(id string) (*AIModel, bool) {
	models := m.loadModels()
	idx := slices.IndexFunc(models, func(v AIModel) bool { return v.ID == id })
	if idx == -1 {
		return nil, false
	}
	return &models[idx], true
}

func (m *ModelsIndex) GetModelsByBrick(brickName string) []AIModel {
	var matches []AIModel
	models := m.loadModels()
	for _, model := range models {
		if slices.ContainsFunc(model.Bricks, func(b BrickConfig) bool { return b.ID == brickName }) {
			matches = append(matches, model)
		}
	}
	return matches
}

func (m *ModelsIndex) GetModelsByBricks(bricks []string) []AIModel {
	var matchingModels []AIModel
	for _, model := range m.loadModels() {
		if slices.ContainsFunc(model.Bricks, func(brick BrickConfig) bool {
			return slices.Contains(bricks, brick.ID)
		}) {
			matchingModels = append(matchingModels, model)
		}
	}
	return matchingModels
}

func (m *ModelsIndex) loadModels() []AIModel {
	eimodels, err := loadCustomModels(m.modelsDir)
	if err != nil {
		slog.Error("cannot load edge impulse custom models", "err", err)
	}
	return append(m.InternalModels, eimodels...)
}

func Load(dir *paths.Path, modelsDir *paths.Path) (*ModelsIndex, error) {
	if dir == nil && modelsDir == nil {
		return &ModelsIndex{}, errors.New("either dir or modelsDir must be provided")
	}
	models, err := loadInternalModels(dir)
	if err != nil {
		return nil, err
	}

	return &ModelsIndex{InternalModels: models, modelsDir: modelsDir}, nil
}

func loadInternalModels(dir *paths.Path) ([]AIModel, error) {
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
			model.IsInternal = true
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
		models = append(models, AIModel{
			ID:                m.ModelDescriptor.ID,
			Name:              m.ModelDescriptor.Name,
			ModuleDescription: m.ModelDescriptor.Description,
			Bricks: f.Map(m.ModelDescriptor.Bricks, func(b custommodel.BrickConfig) BrickConfig {
				return BrickConfig(b)
			}),
			Metadata:        m.ModelDescriptor.Metadata,
			ModelFolderPath: m.FullPath,
			IsInternal:      false,
		})
	}

	return models, nil
}
