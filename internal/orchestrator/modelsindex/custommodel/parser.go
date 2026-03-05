// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package custommodel

import (
	"fmt"
	"strings"

	"github.com/arduino/go-paths-helper"
	"github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

type ModelDescriptor struct {
	ID          string            `yaml:"id"`
	Name        string            `yaml:"name"`
	Runner      string            `yaml:"runner"`
	Description string            `yaml:"description"`
	Bricks      []BrickConfig     `yaml:"bricks"`
	Metadata    map[string]string `yaml:"metadata,omitempty"`
}

type BrickConfig struct {
	ID                 string            `yaml:"id"`
	ModelConfiguration map[string]string `yaml:"model_configuration,omitempty"`
}

// ParseModelDescriptorFile reads a model descriptor file
func ParseModelDescriptorFile(file *paths.Path) (ModelDescriptor, error) {
	f, err := file.Open()
	if err != nil {
		return ModelDescriptor{}, fmt.Errorf("cannot open file: %w", err)
	}
	defer f.Close()
	descriptor := ModelDescriptor{}
	if err := yaml.NewDecoder(f).Decode(&descriptor); err != nil {
		return ModelDescriptor{}, fmt.Errorf("cannot decode descriptor: %w", err)
	}
	return descriptor, nil
}

func (a *ModelDescriptor) Validate() error {
	if a.ID == "" {
		return fmt.Errorf("invalid model descriptor: id is empty")
	}
	if a.Name == "" {
		return fmt.Errorf("invalid model descriptor: name is empty")
	}
	source, ok := a.Metadata["source"]
	if !ok {
		return nil // source is optional
	}

	switch source {
	case "edgeimpulse":
		return validateEdgeImpulseMetadata(a.Metadata)
	default:
		return fmt.Errorf("invalid model descriptor: unsupported source '%s'", source)
	}
}

func (a *ModelDescriptor) CheckEdgeImpulseBricks(bricksIndex *bricksindex.BricksIndex) error {
	for _, brickConfig := range a.Bricks {
		brick, ok := bricksIndex.FindBrickByID(brickConfig.ID)
		if !ok {
			return fmt.Errorf("invalid model descriptor: brick with ID '%s' not found", brickConfig.ID)
		}

		for _, variable := range brick.Variables {
			if strings.HasPrefix(variable.Name, "EI_") && strings.HasSuffix(variable.Name, "_MODEL") {
				if val, ok := brickConfig.ModelConfiguration[variable.Name]; !ok || val == "" {
					return fmt.Errorf("invalid model descriptor: missing model configuration for variable '%s' in brick '%s'", variable.Name, brickConfig.ID)
				}
			}
		}
	}
	return nil
}

func validateEdgeImpulseMetadata(metadata map[string]string) error {
	requiredFields := []string{
		"ei-project-id",
		"ei-impulse-id",
		"ei-impulse-name",
		"ei-deployment-version",
	}
	for _, field := range requiredFields {
		if val, ok := metadata[field]; !ok || val == "" {
			return fmt.Errorf("invalid Edge Impulse metadata: missing required field '%s'", field)
		}
	}

	if metadata["ei-model-type"] != "float32" {
		return fmt.Errorf("invalid Edge Impulse metadata: unsupported model type")
	}

	if metadata["ei-engine"] != "tflite" {
		return fmt.Errorf("invalid Edge Impulse metadata: unsupported engine")
	}

	return nil
}
