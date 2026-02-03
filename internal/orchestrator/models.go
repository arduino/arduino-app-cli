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
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/docker/cli/cli/command"
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

	// Validate if the model is currently in use or referenced.
	// Both checks are performed simultaneously to support the "force" flag logic.
	// This allows the user to see both issues before deciding to use the flag
	// preventing the second error from being masked.
	running, err := isModelUsedInRunningApp(ctx, dockerClient, id)
	if err != nil {
		return fmt.Errorf("could not check if the model is currenlty in use by an app: %w", err)
	}
	references, err := isModelReferencedInApp(ctx, dockerClient, cfg, idProvider, id)
	if err != nil {
		return err
	}

	var errs []error
	if running != "" {
		errs = append(errs, fmt.Errorf("the model %s is in use by the %s app; stop the application before removing the model", id, running))
	}

	// Note: app.yaml files referencing the model will not be modified, resulting
	// in dangling model pointers. An existing check at app startup ensures the user
	// is notified of the missing model.
	if len(references) != 0 {
		errs = append(errs, fmt.Errorf("the model is referenced by bricks belonging to the following apps: %s", strings.Join(references, ", ")))
	}

	if !force && len(errs) > 0 {
		return fmt.Errorf("%w: %w", errors.Join(errs...), ErrConflict)
	}

	if res.ModelFolderPath == nil {
		details := fmt.Sprintf("unexpected null path for model %s", res.ModelFolderPath.String())
		slog.Warn(details)
		return nil
	}

	if err := res.ModelFolderPath.RemoveAll(); err != nil {
		return fmt.Errorf("error removing path %s", res.ModelFolderPath.String())
	}

	return nil
}

func isModelUsedInRunningApp(ctx context.Context, dockerClient command.Cli, modelId string) (string, error) {
	runningApp, err := getRunningApp(ctx, dockerClient.Client())
	if err != nil {
		return "", err
	}
	for _, b := range runningApp.Descriptor.Bricks {
		if b.Model == modelId {
			return runningApp.Name, nil
		}
	}
	return "", nil
}

func isModelReferencedInApp(ctx context.Context, dockerClient command.Cli, cfg config.Configuration, idProvider *app.IDProvider, modelId string) ([]string, error) {
	apps, err := ListApps(ctx, dockerClient, ListAppRequest{
		ShowExamples:                   true,
		ShowApps:                       true,
		IncludeNonStandardLocationApps: true,
	},
		idProvider, cfg)

	if err != nil {
		return nil, err
	}

	references := make(map[string]struct{})
	for _, a := range apps.Apps {
		app, err := app.Load(a.ID.ToPath())
		if err != nil {
			slog.Warn("Unable to load app", slog.Any("application name", a.Name))
			continue
		}
		for _, b := range app.Descriptor.Bricks {
			if b.Model == modelId {
				references[app.Name] = struct{}{}
			}
		}
	}

	return slices.Collect(maps.Keys(references)), nil
}
