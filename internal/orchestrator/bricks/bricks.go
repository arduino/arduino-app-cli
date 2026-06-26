// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricks

import (
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/fatomic"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

var (
	ErrBrickNotFound   = errors.New("brick not found")
	ErrCannotSaveBrick = errors.New("cannot save brick instance")
	ErrBrickNotLocal   = errors.New("brick is not a local brick")
	ErrBrickIDConflict = errors.New("a brick with the new id already exists")
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

func (s *Service) AppBrickInstancesList(a *app.ArduinoApp, platform platform.Platform) (AppBrickInstancesResult, error) {
	res := AppBrickInstancesResult{BrickInstances: make([]BrickInstance, len(a.Descriptor.Bricks))}
	for i, brickInstance := range a.Descriptor.Bricks {
		brick, found := s.bricksIndex.WithAppBricks(a.LocalBricks).FindBrickByID(brickInstance.ID)
		if !found {
			res.BrickInstances[i] = BrickInstance{
				ID:     brickInstance.ID,
				Name:   brickInstance.ID, // using the ID as name to avoid empty UI element
				Status: "not_found",
			}
			continue
		}

		variablesMap, configVariables := getInstanceBrickConfigVariableDetails(brick, brickInstance.Variables)

		res.BrickInstances[i] = BrickInstance{
			ID:              brick.ID,
			Name:            brick.Name,
			Author:          brick.Source,
			Category:        brick.Category,
			Status:          "installed",
			RequireModel:    brick.RequireModel,
			ModelID:         cmp.Or(brickInstance.Model, brick.GetModelNameByBoard(platform.BoardName)),
			Variables:       variablesMap,
			ConfigVariables: configVariables,
			CompatibleModels: f.Map(s.modelsIndex.GetModelsByBrick(brick.ID), func(m modelsindex.AIModelLite) AIModel {
				return AIModel{
					ID:          m.ID,
					Name:        m.Name,
					Description: m.Description,
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
		CompatibleModels: f.Map(s.modelsIndex.GetModelsByBrick(brick.ID), func(m modelsindex.AIModelLite) AIModel {
			return AIModel{
				ID:   m.ID,
				Name: m.Name,
				// TODO: deprecated field, remove in future versions
				Description: m.Description,
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
		slog.Warn("cannot open readme for brick", "brickID", brick.ID, "error", err.Error())
	}

	var apiDocsPath string
	if p, ok := brick.GetApiDocPath(); ok {
		apiDocsPath = p.String()
	} else {
		slog.Warn("cannot load API doc", "brickID", brick.ID)
	}

	examplePaths, err := brick.GetExamplesPath()
	if err != nil {
		slog.Warn("cannot load example for brick", "brickID", brick.ID, "error", err.Error())
	}
	codeExamples := f.Map(examplePaths, func(p *paths.Path) CodeExample {
		return CodeExample{
			Path: p.String(),
		}
	})

	usedByApps, err := getUsedByApps(cfg, brick.ID, idProvider)
	if err != nil {
		slog.Warn("unable to get used by apps for brick", "brickID", brick.ID, "error", err.Error())
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
		ApiDocsPath:  apiDocsPath,
		CodeExamples: codeExamples,
		UsedByApps:   usedByApps,
		CompatibleModels: f.Map(s.modelsIndex.GetModelsByBrick(brick.ID), func(m modelsindex.AIModelLite) AIModel {
			return AIModel{
				ID:          m.ID,
				Name:        m.Name,
				Description: m.Description,
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
		if !s.modelsIndex.IsModelSupportedByBrick(*req.Model, req.ID) {
			return fmt.Errorf("model %s does not exsist", *req.Model)
		}
		brickInstance.Model = *req.Model
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
		if !s.modelsIndex.IsModelSupportedByBrick(*req.Model, req.ID) {
			return fmt.Errorf("model %s is not supported by brick %q", *req.Model, req.ID)
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
	if !slices.ContainsFunc(appCurrent.Descriptor.Bricks, func(b app.Brick) bool { return b.ID == id }) {
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

// LocalBrickRename renames a local brick by changing its ID, folder name, and display name.
// The newID is derived from the newName by the caller (handler layer).
func (s *Service) LocalBrickRename(appCurrent *app.ArduinoApp, oldID, newID, newName string) (_ LocalBrickRenameResult, _err error) {
	if oldID == newID {
		return LocalBrickRenameResult{}, fmt.Errorf("new brick id %q is the same as the current one", newID)
	}

	localBrickIdx := slices.IndexFunc(appCurrent.LocalBricks, func(b bricksindex.Brick) bool { return b.ID == oldID })
	if localBrickIdx == -1 {
		if _, found := s.bricksIndex.FindBrickByID(oldID); found {
			return LocalBrickRenameResult{}, ErrBrickNotLocal
		}
		return LocalBrickRenameResult{}, ErrBrickNotFound
	}

	if _, exist := s.bricksIndex.WithAppBricks(appCurrent.LocalBricks).FindBrickByID(newID); exist {
		return LocalBrickRenameResult{}, ErrBrickIDConflict
	}

	oldBrickPath := appCurrent.LocalBricks[localBrickIdx].FullPath
	newBrickPath := appCurrent.LocalBricks[localBrickIdx].FullPath.Parent().Join(newID)

	if err := oldBrickPath.Rename(newBrickPath); err != nil {
		return LocalBrickRenameResult{}, fmt.Errorf("cannot rename brick folder: %w", err)
	}
	// Rollback to old name in case of any error in the following steps.
	defer func() {
		if _err != nil {
			_ = newBrickPath.Rename(oldBrickPath)
		}
	}()

	configPath := newBrickPath.Join("brick_config.yaml")
	oldBrickConfigContent, err := os.ReadFile(configPath.String())
	if err != nil {
		return LocalBrickRenameResult{}, fmt.Errorf("cannot read brick_config.yaml: %w", err)
	}
	if err := updateBrickConfig(configPath, newID, newName); err != nil {
		return LocalBrickRenameResult{}, fmt.Errorf("cannot update brick_config.yaml: %w", err)
	}
	// Rollback brick_config.yaml in case of any error in the following steps.
	defer func() {
		if _err != nil {
			_ = fatomic.WriteFile(configPath.String(), oldBrickConfigContent, os.FileMode(0644))
		}
	}()

	if i := slices.IndexFunc(appCurrent.Descriptor.Bricks, func(b app.Brick) bool { return b.ID == oldID }); i != -1 {
		appCurrent.Descriptor.Bricks[i].ID = newID

		// Rollback to old ID in case of any error in the following steps.
		defer func() {
			if _err != nil && i != -1 {
				appCurrent.Descriptor.Bricks[i].ID = oldID
				_ = appCurrent.Save()
			}
		}()

		if err := appCurrent.Save(); err != nil {
			return LocalBrickRenameResult{}, fmt.Errorf("cannot save app: %w", err)
		}
	}

	return LocalBrickRenameResult{ID: newID}, nil
}

func updateBrickConfig(brickConfigPath *paths.Path, newID, newName string) error {
	content, err := os.ReadFile(brickConfigPath.String())
	if err != nil {
		return fmt.Errorf("cannot read brick_config.yaml: %w", err)
	}

	var brick bricksindex.Brick
	if err := yaml.Unmarshal(content, &brick); err != nil {
		return fmt.Errorf("cannot unmarshal brick_config.yaml: %w", err)
	}

	brick.ID = newID
	brick.Name = newName

	updated, err := yaml.Marshal(brick)
	if err != nil {
		return fmt.Errorf("cannot marshal brick_config.yaml: %w", err)
	}

	if err := fatomic.WriteFile(brickConfigPath.String(), updated, os.FileMode(0644)); err != nil {
		return fmt.Errorf("cannot write brick_config.yaml: %w", err)
	}
	return nil
}
