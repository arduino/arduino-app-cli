// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package custommodel

import (
	"errors"
	"fmt"

	"github.com/arduino/go-paths-helper"
	"github.com/goccy/go-yaml"
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
	var err error
	if a.ID == "" {
		err = errors.Join(err, fmt.Errorf("invalid model descriptor: id is empty"))
	}
	if a.Name == "" {
		err = errors.Join(err, fmt.Errorf("invalid model descriptor: name is empty"))
	}
	source, ok := a.Metadata["source"]
	if ok {
		switch source {
		case "edgeimpulse":
			err = errors.Join(err, validateEdgeImpulseMetadata(a.Metadata))
		default:
			err = errors.Join(err, fmt.Errorf("invalid model descriptor: unsupported source '%s'", source))
		}
	}
	return err
}

// validateEdgeImpulseMetadata checks that all metadata fields required by App Lab are present and valid.
func validateEdgeImpulseMetadata(metadata map[string]string) error {
	requiredFields := []string{
		"ei-project-id",
		"ei-impulse-id",
		"ei-impulse-name",
		"ei-deployment-version",
	}

	var err error
	for _, field := range requiredFields {
		if val, ok := metadata[field]; !ok || val == "" {
			err = errors.Join(err, fmt.Errorf("invalid Edge Impulse metadata: missing required field '%s'", field))
		}
	}
	if metadata["ei-model-type"] != "float32" {
		err = errors.Join(err, fmt.Errorf("invalid Edge Impulse metadata: unsupported model type"))
	}
	if metadata["ei-engine"] != "tflite" {
		err = errors.Join(err, fmt.Errorf("invalid Edge Impulse metadata: unsupported engine"))
	}
	return err
}
