// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

type ExampleResponse struct {
	CoreAndFoundational []CategoryExamples `json:"coreAndFoundational"`
	Bricks              []BrickExamples    `json:"bricks"`
}

type CategoryExamples struct {
	Category string    `json:"category"`
	Examples []Example `json:"examples"`
}

type BrickExamples struct {
	Brick    string    `json:"brick"`
	Examples []Example `json:"examples"`
}

type Example struct {
	ID          string `json:"id"`
	EncodedID   string `json:"encoded_id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

func GetExamples(cfg config.Configuration, idProvider *appid.Provider) (ExampleResponse, error) {
	data, err := getExampleFile(cfg).ReadFile()
	if err != nil {
		return ExampleResponse{}, fmt.Errorf("error reading json: %w", err)
	}

	var examples ExampleResponse
	err = json.Unmarshal(data, &examples)
	if err != nil {
		return ExampleResponse{}, fmt.Errorf("could not parse json: %w", err)
	}

	for i := range examples.CoreAndFoundational {
		retrieveExamplesInfo(examples.CoreAndFoundational[i].Examples, idProvider)
	}

	for i := range examples.Bricks {
		retrieveExamplesInfo(examples.Bricks[i].Examples, idProvider)
	}

	return examples, nil
}

func getExampleFile(cfg config.Configuration) *paths.Path {
	return cfg.ExamplesBaseDir().Join("examples.json")
}

func retrieveExamplesInfo(examples []Example, idProvider *appid.Provider) {
	for i := range examples {
		example := &examples[i]
		id, err := idProvider.ParseID(example.ID)
		if err != nil {
			slog.Warn("Id is not valid", slog.String("id", example.ID))
			continue
		}

		loadedApp, err := app.Load(id.ToPath())
		if err != nil {
			slog.Warn("App referenced in examples not found", slog.String("id", example.ID))
			continue
		}

		example.Name = loadedApp.Name
		example.Description = loadedApp.Descriptor.Description
		example.EncodedID = id.String()
	}
}
