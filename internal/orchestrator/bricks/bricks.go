// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package bricks

import (
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/arduino/go-paths-helper"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
)

var (
	ErrBrickNotFound   = errors.New("brick not found")
	ErrCannotSaveBrick = errors.New("cannot save brick instance")
)

type Service struct {
	modelsIndex *modelsindex.ModelsIndex
	bricksIndex *bricksindex.BricksIndex
}

func NewService(
	modelsIndex *modelsindex.ModelsIndex,
	bricksIndex *bricksindex.BricksIndex,
) *Service {
	return &Service{
		modelsIndex: modelsIndex,
		bricksIndex: bricksIndex,
	}
}

func (s *Service) List() (BrickListResult, error) {
	res := BrickListResult{Bricks: make([]BrickListItem, len(s.bricksIndex.ListBricks()))}
	for i, brick := range s.bricksIndex.ListBricks() {
		res.Bricks[i] = BrickListItem{
			ID:           brick.ID,
			Name:         brick.Name,
			Author:       brick.Source,
			Description:  brick.Description,
			Category:     brick.Category,
			Status:       "installed",
			RequireModel: brick.RequireModel,
		}
	}
	return res, nil
}

func (s *Service) AppBrickInstancesList(a *app.ArduinoApp) (AppBrickInstancesResult, error) {
	res := AppBrickInstancesResult{BrickInstances: make([]BrickInstance, len(a.Descriptor.Bricks))}
	for i, brickInstance := range a.Descriptor.Bricks {
		brick, found := s.bricksIndex.WithAppBricks(a.LocalBricks).FindBrickByID(brickInstance.ID)
		if !found {
			return AppBrickInstancesResult{}, fmt.Errorf("brick not found with id %s", brickInstance.ID)
		}

		variablesMap, configVariables := getInstanceBrickConfigVariableDetails(brick, brickInstance.Variables)

		res.BrickInstances[i] = BrickInstance{
			ID:              brick.ID,
			Name:            brick.Name,
			Author:          brick.Source,
			Category:        brick.Category,
			Status:          "installed",
			RequireModel:    brick.RequireModel,
			ModelID:         cmp.Or(brickInstance.Model, brick.ModelName),
			Variables:       variablesMap,
			ConfigVariables: configVariables,
			CompatibleModels: f.Map(s.modelsIndex.GetModelsByBrick(brick.ID), func(m modelsindex.AIModel) AIModel {
				return AIModel{
					ID:          m.ID,
					Name:        m.Name,
					Description: m.ModuleDescription,
				}
			}),
		}

	}
	return res, nil
}

func (s *Service) AppBrickInstanceDetails(a *app.ArduinoApp, brickID string) (BrickInstance, error) {
	bricksindex := s.bricksIndex.WithAppBricks(a.LocalBricks)
	brick, found := bricksindex.FindBrickByID(brickID)
	if !found {
		return BrickInstance{}, ErrBrickNotFound
	}
	// Check if the brick is already added in the app
	brickIndex := slices.IndexFunc(a.Descriptor.Bricks, func(b app.Brick) bool { return b.ID == brickID })
	if brickIndex == -1 {
		return BrickInstance{}, fmt.Errorf("brick %s not added in the app", brickID)
	}

	variables, configVariables := getInstanceBrickConfigVariableDetails(brick, a.Descriptor.Bricks[brickIndex].Variables)

	var readme string
	if r, err := brick.GetReadmeFile(); err == nil {
		readme = r
	} else {
		slog.Warn("cannot open readme for brick", "brickID", brick.ID, "error", err.Error())
	}

	return BrickInstance{
		ID:              brickID,
		Name:            brick.Name,
		Author:          brick.Source,
		Category:        brick.Category,
		Status:          "installed", // For now every Arduino brick are installed
		RequireModel:    brick.RequireModel,
		Variables:       variables,
		ConfigVariables: configVariables,
		ModelID:         cmp.Or(a.Descriptor.Bricks[brickIndex].Model, brick.ModelName),
		CompatibleModels: f.Map(s.modelsIndex.GetModelsByBrick(brick.ID), func(m modelsindex.AIModel) AIModel {
			return AIModel{
				ID:          m.ID,
				Name:        m.Name,
				Description: m.ModuleDescription,
			}
		}),
		Readme: readme,
	}, nil
}

func getInstanceBrickConfigVariableDetails(
	brick *bricksindex.Brick, userVariables map[string]string,
) (map[string]string, []BrickConfigVariable) {
	variablesMap := make(map[string]string, len(brick.Variables))
	variableDetails := make([]BrickConfigVariable, 0, len(brick.Variables))

	for _, v := range brick.Variables {
		if v.Hidden {
			continue
		}
		finalValue := v.DefaultValue

		userValue, ok := userVariables[v.Name]
		if ok {
			finalValue = userValue
		}
		variablesMap[v.Name] = finalValue

		variableDetails = append(variableDetails, BrickConfigVariable{
			Name:        v.Name,
			Value:       finalValue,
			Description: v.Description,
			Required:    v.IsRequired(),
		})
	}

	return variablesMap, variableDetails
}

func (s *Service) BricksDetails(id string, idProvider *app.IDProvider,
	cfg config.Configuration) (BrickDetailsResult, error) {
	brick, found := s.bricksIndex.FindBrickByID(id)
	if !found {
		return BrickDetailsResult{}, ErrBrickNotFound
	}

	readme, err := brick.GetReadmeFile()
	if err != nil {
		return BrickDetailsResult{}, fmt.Errorf("cannot open docs for brick %s: %w", id, err)
	}

	apiDocsPath, found := brick.GetApiDocPath()
	if !found {
		return BrickDetailsResult{}, fmt.Errorf("cannot open api-docs for brick %s", id)
	}

	examplePaths, err := brick.GetExamplesPath()
	if err != nil {
		return BrickDetailsResult{}, fmt.Errorf("cannot open code examples for brick %s: %w", id, err)
	}
	codeExamples := f.Map(examplePaths, func(p *paths.Path) CodeExample {
		return CodeExample{
			Path: p.String(),
		}
	})

	usedByApps, err := getUsedByApps(cfg, brick.ID, idProvider)
	if err != nil {
		return BrickDetailsResult{}, fmt.Errorf("unable to get used by apps: %w", err)
	}

	variables, configVariables := getBrickConfigVariableDetails(brick)

	return BrickDetailsResult{
		ID:           id,
		Name:         brick.Name,
		Author:       brick.Source,
		Description:  brick.Description,
		Category:     brick.Category,
		RequireModel: brick.RequireModel,
		Status:       "installed", // For now every Arduino brick are installed
		Variables:    variables,
		Readme:       readme,
		ApiDocsPath:  apiDocsPath.String(),
		CodeExamples: codeExamples,
		UsedByApps:   usedByApps,
		CompatibleModels: f.Map(s.modelsIndex.GetModelsByBrick(brick.ID), func(m modelsindex.AIModel) AIModel {
			return AIModel{
				ID:          m.ID,
				Name:        m.Name,
				Description: m.ModuleDescription,
			}
		}),
		ConfigVariables: configVariables,
	}, nil
}

func getBrickConfigVariableDetails(
	brick *bricksindex.Brick) (map[string]BrickVariable, []BrickConfigVariable) {
	variablesMap := make(map[string]BrickVariable, len(brick.Variables))
	variableDetails := make([]BrickConfigVariable, 0, len(brick.Variables))

	for _, v := range brick.Variables {
		if v.Hidden {
			continue
		}
		variablesMap[v.Name] = BrickVariable{
			DefaultValue: v.DefaultValue,
			Description:  v.Description,
			Required:     v.IsRequired(),
		}

		variableDetails = append(variableDetails, BrickConfigVariable{
			Name:        v.Name,
			Value:       v.DefaultValue,
			Description: v.Description,
			Required:    v.IsRequired(),
		})
	}

	return variablesMap, variableDetails
}

func getUsedByApps(cfg config.Configuration, brickId string, idProvider *app.IDProvider) ([]AppReference, error) {
	var appPaths paths.PathList

	pathsToExplore := paths.NewPathList()
	pathsToExplore.Add(cfg.ExamplesDir())
	pathsToExplore.Add(cfg.AppsDir())
	for _, p := range pathsToExplore {
		res, err := app.FindAppsInFolder(p)
		if err != nil {
			slog.Error("unable to list apps", slog.String("error", err.Error()))
			return []AppReference{}, err
		}
		appPaths.AddAllMissing(res)
	}

	usedByApps := []AppReference{}
	for _, appPath := range appPaths {
		app, err := app.Load(appPath)
		if err != nil {
			// we are not considering the broken apps
			slog.Warn("unable to parse app.yaml, skipping", "path", appPath.String(), "error", err.Error())
			continue
		}

		for _, b := range app.Descriptor.Bricks {
			if b.ID == brickId {
				id, err := idProvider.IDFromPath(app.FullPath)
				if err != nil {
					return []AppReference{}, fmt.Errorf("failed to get app ID for %s: %w", app.FullPath, err)
				}
				usedByApps = append(usedByApps, AppReference{
					Name: app.Name,
					ID:   id.String(),
					Icon: app.Descriptor.Icon,
				})
				break
			}
		}
	}
	return usedByApps, nil
}

type BrickCreateUpdateRequest struct {
	ID        string            `json:"-"`
	Model     *string           `json:"model"`
	Variables map[string]string `json:"variables,omitempty"`
}

func (s *Service) BrickCreate(
	req BrickCreateUpdateRequest,
	appCurrent app.ArduinoApp,
) error {
	brick, present := s.bricksIndex.WithAppBricks(appCurrent.LocalBricks).FindBrickByID(req.ID)
	if !present {
		return fmt.Errorf("brick %q not found", req.ID)
	}

	for name, reqValue := range req.Variables {
		value, exist := brick.GetVariable(name)
		if !exist {
			return fmt.Errorf("variable %q does not exist on brick %q", name, brick.ID)
		}
		if value.IsRequired() && reqValue == "" {
			return fmt.Errorf("required variable %q cannot be empty", name)
		}
	}

	for _, brickVar := range brick.Variables {
		if brickVar.IsRequired() {
			if _, exist := req.Variables[brickVar.Name]; !exist {
				slog.Warn("[Skip] a required variable is not set by user", "variable", brickVar.Name, "brick", brickVar.Name)
			}
		}
	}

	brickIndex := -1
	var brickInstance app.Brick

	for index, b := range appCurrent.Descriptor.Bricks {
		if b.ID == req.ID {
			brickIndex = index
			brickInstance = b
			break
		}
	}

	brickInstance.ID = req.ID

	if req.Model != nil {
		models := s.modelsIndex.GetModelsByBrick(brickInstance.ID)
		idx := slices.IndexFunc(models, func(m modelsindex.AIModel) bool { return m.ID == *req.Model })
		if idx == -1 {
			return fmt.Errorf("model %s does not exsist", *req.Model)
		}
		brickInstance.Model = models[idx].ID
	}
	brickInstance.Variables = req.Variables

	if brickIndex == -1 {
		appCurrent.Descriptor.Bricks = append(appCurrent.Descriptor.Bricks, brickInstance)
	} else {
		appCurrent.Descriptor.Bricks[brickIndex] = brickInstance
	}

	err := appCurrent.Save()
	if err != nil {
		return fmt.Errorf("cannot save brick instance with id %s", req.ID)
	}
	return nil
}

func (s *Service) BrickUpdate(
	req BrickCreateUpdateRequest,
	appCurrent app.ArduinoApp,
) error {
	brickFromIndex, present := s.bricksIndex.WithAppBricks(appCurrent.LocalBricks).FindBrickByID(req.ID)
	if !present {
		return fmt.Errorf("brick %q not found into the brick index", req.ID)
	}

	brickPosition := slices.IndexFunc(appCurrent.Descriptor.Bricks, func(b app.Brick) bool { return b.ID == req.ID })
	if brickPosition == -1 {
		return fmt.Errorf("brick %q not found into the bricks of the app", req.ID)
	}

	brickVariables := appCurrent.Descriptor.Bricks[brickPosition].Variables
	if len(brickVariables) == 0 {
		brickVariables = make(map[string]string)
	}
	brickModel := appCurrent.Descriptor.Bricks[brickPosition].Model

	if req.Model != nil && *req.Model != brickModel {
		models := s.modelsIndex.GetModelsByBrick(req.ID)
		idx := slices.IndexFunc(models, func(m modelsindex.AIModel) bool { return m.ID == *req.Model })
		if idx == -1 {
			return fmt.Errorf("model %s does not exsist", *req.Model)
		}
		brickModel = *req.Model
	}

	for name, updateValue := range req.Variables {
		value, exist := brickFromIndex.GetVariable(name)
		if !exist {
			return fmt.Errorf("variable %q does not exist on brick %q", name, brickFromIndex.ID)
		}
		if value.IsRequired() && updateValue == "" {
			return fmt.Errorf("required variable %q cannot be empty", name)
		}
		updated := false
		for _, v := range brickVariables {
			if v == name {
				brickVariables[name] = updateValue
				updated = true
				break
			}
		}
		if !updated {
			brickVariables[name] = updateValue
		}
	}

	appCurrent.Descriptor.Bricks[brickPosition].Model = brickModel
	appCurrent.Descriptor.Bricks[brickPosition].Variables = brickVariables

	err := appCurrent.Save()
	if err != nil {
		return fmt.Errorf("cannot save brick instance with id %s", req.ID)
	}
	return nil

}

func (s *Service) BrickDelete(
	appCurrent *app.ArduinoApp,
	id string,
) error {
	if _, present := s.bricksIndex.WithAppBricks(appCurrent.LocalBricks).FindBrickByID(id); !present {
		return ErrBrickNotFound
	}

	appCurrent.Descriptor.Bricks = slices.DeleteFunc(appCurrent.Descriptor.Bricks, func(b app.Brick) bool {
		return b.ID == id
	})

	if err := appCurrent.Save(); err != nil {
		return ErrCannotSaveBrick
	}
	return nil
}
