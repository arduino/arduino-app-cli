// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricks

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"slices"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"
	"go.bug.st/f"

	apimodels "github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/fatomic"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/secrets"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

var (
	ErrBrickNotFound            = errors.New("brick not found")
	ErrCannotSaveBrick          = errors.New("cannot save brick instance")
	ErrBrickNotLocal            = errors.New("brick is not a local brick")
	ErrBrickIDConflict          = errors.New("a brick with the new id already exists")
	ErrCannotAccessSecretsStore = errors.New("can't access secrets store")
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

func (s *Service) List() BrickListResult {
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
	return res
}

func (s *Service) AppBrickInstancesList(ctx context.Context, a *app.ArduinoApp, secretsStore *secrets.Store) (AppBrickInstancesResult, error) {
	secretValues, err := getSecretValues(secretsStore)
	if err != nil {
		return AppBrickInstancesResult{}, ErrCannotAccessSecretsStore
	}
	res := AppBrickInstancesResult{BrickInstances: make([]BrickInstance, len(a.Descriptor.Bricks))}
	// One lookup for every brick instance, rather than a listing each.
	models := s.modelsIndex.NewLookup()
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

		variablesMap, configVariables := getInstanceBrickConfigVariableDetails(brick, populateSecretVariablesFromStore(brick, brickInstance.Variables, secretValues))

		res.BrickInstances[i] = BrickInstance{
			ID:               brick.ID,
			Name:             brick.Name,
			Author:           brick.Source,
			Category:         brick.Category,
			Status:           "installed",
			RequireModel:     brick.RequireModel,
			ModelID:          apimodels.EncodeModelID(cmp.Or(brickInstance.Model, brick.ModelName)),
			Variables:        variablesMap,
			ConfigVariables:  configVariables,
			CompatibleModels: compatibleModels(ctx, models, brick.ID),
		}

	}
	return res, nil
}

// compatibleModels lists the models the brick can use, with the ids encoded.
func compatibleModels(ctx context.Context, models *modelsindex.Lookup, brickID string) []AIModel {
	matches, err := models.ByBrick(ctx, brickID)
	if err != nil {
		slog.Warn("cannot get models info, brick compatibility list may be incomplete", "brick", brickID, "err", err)
	}
	return f.Map(matches, func(m modelsindex.AIModelLite) AIModel {
		return AIModel{
			ID:   apimodels.EncodeModelID(m.ID),
			Name: m.Name,
			// TODO: deprecated field, remove in future versions
			Description: m.Description,
		}
	})
}

func (s *Service) AppBrickInstanceDetails(ctx context.Context, a *app.ArduinoApp, brickID string, secretsStore *secrets.Store) (BrickInstance, error) {
	secretValues, err := getSecretValues(secretsStore)
	if err != nil {
		return BrickInstance{}, ErrCannotAccessSecretsStore
	}
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

	variables, configVariables := getInstanceBrickConfigVariableDetails(brick, populateSecretVariablesFromStore(brick, a.Descriptor.Bricks[brickIndex].Variables, secretValues))

	var readme string
	if r, err := brick.GetReadmeFile(); err == nil {
		readme = r
	} else {
		slog.Warn("cannot open readme for brick", slog.String("brickID", brick.ID), slog.Any("error", err.Error()))
	}

	return BrickInstance{
		ID:               brickID,
		Name:             brick.Name,
		Author:           brick.Source,
		Category:         brick.Category,
		Status:           "installed", // For now every Arduino brick are installed
		RequireModel:     brick.RequireModel,
		Variables:        variables,
		ConfigVariables:  configVariables,
		ModelID:          apimodels.EncodeModelID(cmp.Or(a.Descriptor.Bricks[brickIndex].Model, brick.ModelName)),
		CompatibleModels: compatibleModels(ctx, s.modelsIndex.NewLookup(), brick.ID),
		Readme:           readme,
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

func (s *Service) BricksDetails(ctx context.Context, id string, idProvider *appid.Provider,
	cfg config.Configuration, platform platform.Platform) (BrickDetailsResult, error) {
	brick, found := s.bricksIndex.FindBrickByID(id)
	if !found {
		return BrickDetailsResult{}, ErrBrickNotFound
	}

	readme, err := brick.GetReadmeFile()
	if err != nil {
		slog.Warn("cannot open readme for brick", slog.String("brickID", brick.ID), slog.Any("error", err.Error()))
	}

	var apiDocsPath string
	if p, ok := brick.GetApiDocPath(); ok {
		apiDocsPath = p.String()
	} else {
		slog.Warn("cannot load API doc", slog.String("brickID", brick.ID))
	}

	brickNamespace, brickName, err := bricksindex.ParseBrickID(brick.ID)
	if err != nil {
		slog.Warn("invalid brick id", "brickID", brick.ID, "error", err.Error())
	}
	codeExamples := getBrickExamplesInfo(cfg, idProvider, brickNamespace, brickName)

	usedByApps, err := getUsedByApps(cfg, brick.ID, idProvider, platform)
	if err != nil {
		slog.Warn("unable to get used by apps for brick", slog.String("brickID", brick.ID), slog.Any("error", err.Error()))
	}

	variables, configVariables := getBrickConfigVariableDetails(brick)

	return BrickDetailsResult{
		ID:                        id,
		Name:                      brick.Name,
		Author:                    brick.Source,
		Description:               brick.Description,
		Category:                  brick.Category,
		RequireModel:              brick.RequireModel,
		Status:                    "installed", // For now every Arduino brick are installed
		Variables:                 variables,
		Readme:                    readme,
		ApiDocsPath:               apiDocsPath,
		CodeExamples:              codeExamples,
		UsedByApps:                usedByApps,
		CompatibleModels:          compatibleModels(ctx, s.modelsIndex.NewLookup(), brick.ID),
		ConfigVariables:           configVariables,
		AIFrameworksCompatibility: brick.AIFrameworksCompatibility,
	}, nil
}

func getBrickExamplesInfo(cfg config.Configuration, idProvider *appid.Provider, brickNamespace string, brickName string) []CodeExample {
	var codeExamples = []CodeExample{}

	examplesPath := cfg.ExamplesBaseDir().Join("bricks", brickNamespace, brickName)

	dirEntries, err := examplesPath.ReadDir()
	if err != nil {
		slog.Warn("cannot read examples directory", slog.Any("error", err.Error()))
		return []CodeExample{}
	}

	for _, brickExamplePath := range dirEntries {
		id, err := idProvider.IDFromPath(brickExamplePath)
		if err != nil {
			slog.Warn("Invalid path", slog.String("brickExamplePath", brickExamplePath.String()))
			continue
		}

		loadedApp, err := app.Load(id.ToPath())
		if err != nil {
			slog.Warn("App referenced in examples not found", slog.String("brickExamplePath", brickExamplePath.String()))
			continue
		}

		mainPy := brickExamplePath.Join("python", "main.py")

		codeExamples = append(codeExamples, CodeExample{
			Path:        mainPy.String(),
			EncodedID:   id.String(),
			Name:        loadedApp.Name,
			Description: loadedApp.Descriptor.Description,
		})
	}

	return codeExamples
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

// Additional core-and-foundational and brick paths are not processed here;
// we do not want them to appear in the brick example list.
func getUsedByApps(cfg config.Configuration, brickId string, idProvider *appid.Provider, platform platform.Platform) ([]AppReference, error) {
	pathsToExplore := paths.NewPathList()
	pathsToExplore.AddAll(cfg.ExamplesDirs(platform))
	pathsToExplore.Add(cfg.AppsDir())
	appPaths, err := app.FindAppsInFolders(pathsToExplore)
	if err != nil {
		slog.Error("unable to list apps", slog.String("error", err.Error()))
		return []AppReference{}, err
	}

	usedByApps := []AppReference{}
	for _, appPath := range appPaths {
		app, err := app.Load(appPath)
		if err != nil {
			// we are not considering the broken apps
			slog.Warn("unable to parse app.yaml, skipping", slog.String("path", appPath.String()), slog.Any("error", err.Error()))
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
	Model     *string           `json:"model" example:"bGxhbWFjcHA6Z2VtbWEtMy0xYi1pdC1RNF8w"`
	Variables map[string]string `json:"variables,omitempty"`
}

func (s *Service) BrickCreate(
	ctx context.Context,
	req BrickCreateUpdateRequest,
	appCurrent app.ArduinoApp,
	secretsStore *secrets.Store,
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
				slog.Warn("[Skip] a required variable is not set by user", slog.String("variable", brickVar.Name), slog.String("brick", brickVar.Name))
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
		model, err := s.modelsIndex.NewLookup().ModelForBrick(ctx, *req.Model, req.ID)
		if err != nil {
			return fmt.Errorf("checking model %s: %w", *req.Model, err)
		}
		if model == nil {
			return fmt.Errorf("model %s does not exsist", *req.Model)
		}
		brickInstance.Model = model.ID
	}
	if secretsStore == nil {
		// Legacy: if there is no secrets store, we treat all variables as
		// non-secret and store them directly in the brick instance.
		brickInstance.Variables = req.Variables

		// TODO: shall we panic here, and assert the presence of a secrets store?
	} else {
		// If there are "legacy" secrets (i.e. secrets that were previously stored directly in the brick instance),
		// we extract them and merge them with the non-secret variables from the request.
		legacySecrets := brickSecretVariables(brick, brickInstance.Variables)

		brickInstance.Variables = brickNonSecretVariables(brick, req.Variables)
		maps.Copy(brickInstance.Variables, legacySecrets)
	}

	if brickIndex == -1 {
		appCurrent.Descriptor.Bricks = append(appCurrent.Descriptor.Bricks, brickInstance)
	} else {
		appCurrent.Descriptor.Bricks[brickIndex] = brickInstance
	}

	if err := appCurrent.Save(); err != nil {
		return fmt.Errorf("cannot save brick instance with id %s: %w", req.ID, err)
	}
	if err := updateSecretValues(secretsStore, brickSecretVariables(brick, req.Variables)); err != nil {
		return fmt.Errorf("cannot save secrets for brick instance with id %s: %w", req.ID, err)
	}
	return nil
}

func (s *Service) BrickUpdate(
	ctx context.Context,
	req BrickCreateUpdateRequest,
	appCurrent app.ArduinoApp,
	secretsStore *secrets.Store,
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
		model, err := s.modelsIndex.NewLookup().ModelForBrick(ctx, *req.Model, req.ID)
		if err != nil {
			return fmt.Errorf("checking model %s: %w", *req.Model, err)
		}
		if model == nil {
			return fmt.Errorf("model %s is not supported by brick %q", *req.Model, req.ID)
		}
		brickModel = model.ID
	}

	for name, updateValue := range req.Variables {
		value, exist := brickFromIndex.GetVariable(name)
		if !exist {
			return fmt.Errorf("variable %q does not exist on brick %q", name, brickFromIndex.ID)
		}
		if value.IsRequired() && updateValue == "" {
			return fmt.Errorf("required variable %q cannot be empty", name)
		}
		if value.Secret && secretsStore != nil {
			continue
		}
		brickVariables[name] = updateValue
	}

	appCurrent.Descriptor.Bricks[brickPosition].Model = brickModel
	appCurrent.Descriptor.Bricks[brickPosition].Variables = brickVariables

	if err := appCurrent.Save(); err != nil {
		return fmt.Errorf("cannot save brick instance with id %s", req.ID)
	}
	if err := updateSecretValues(secretsStore, brickSecretVariables(brickFromIndex, req.Variables)); err != nil {
		return fmt.Errorf("cannot save secrets for brick instance with id %s: %w", req.ID, err)
	}
	return nil

}

func getSecretValues(store *secrets.Store) (map[string]string, error) {
	if store == nil {
		return nil, nil
	}
	return store.Get()
}

func updateSecretValues(secretsStore *secrets.Store, updates map[string]string) error {
	if secretsStore == nil || len(updates) == 0 {
		return nil
	}
	values, err := secretsStore.Get()
	if err != nil {
		return err
	}
	for name, value := range updates {
		values[name] = value
	}
	return secretsStore.Set(values)
}

// brickSecretVariables selects from the given map only the variables that are marked as secret in the brick's definition.
func brickSecretVariables(brick *bricksindex.Brick, variables map[string]string) map[string]string {
	secrets := make(map[string]string)
	for name, value := range variables {
		if variable, found := brick.GetVariable(name); found && variable.Secret {
			secrets[name] = value
		}
	}
	return secrets
}

// brickNonSecretVariables selects from the given map only the variables that are not marked as secret in the brick's definition.
func brickNonSecretVariables(brick *bricksindex.Brick, variables map[string]string) map[string]string {
	values := make(map[string]string)
	for name, value := range variables {
		if variable, found := brick.GetVariable(name); found && !variable.Secret {
			values[name] = value
		}
	}
	return values
}

// populateSecretVariablesFromStore return a copy of the given variables map populated with the indexedBrick secret variables.
// The secret values are taken from the secretVariables map (unless a value is already present in the variables map).
func populateSecretVariablesFromStore(indexedBrick *bricksindex.Brick, variables, secretVariables map[string]string) map[string]string {
	effective := maps.Clone(variables)
	if effective == nil {
		effective = make(map[string]string)
	}
	for _, variable := range indexedBrick.Variables {
		if !variable.Secret {
			continue
		}
		if _, legacyValue := effective[variable.Name]; legacyValue {
			continue
		}
		if value, stored := secretVariables[variable.Name]; stored {
			effective[variable.Name] = value
		}
	}
	return effective
}

// removeUnusedSecretValues removes secret values from the secrets store that are no longer used by any of the given bricks.
func removeUnusedSecretValues(index *bricksindex.BricksIndex, bricks []app.Brick, secretsStore *secrets.Store) error {
	if secretsStore == nil {
		return nil
	}
	values, err := secretsStore.Get()
	if err != nil {
		return err
	}
	used := map[string]bool{}
	for _, brick := range bricks {
		indexedBrick, found := index.FindBrickByID(brick.ID)
		if !found {
			continue
		}
		for _, variable := range indexedBrick.Variables {
			if variable.Secret {
				used[variable.Name] = true
			}
		}
	}
	for name := range values {
		if !used[name] {
			delete(values, name)
		}
	}
	return secretsStore.Set(values)
}

func (s *Service) BrickDelete(
	appCurrent *app.ArduinoApp,
	id string,
	secretsStore *secrets.Store,
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
	if err := removeUnusedSecretValues(s.bricksIndex.WithAppBricks(appCurrent.LocalBricks), appCurrent.Descriptor.Bricks, secretsStore); err != nil {
		return fmt.Errorf("%w: %v", ErrCannotSaveBrick, err)
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
