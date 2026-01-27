package custommodel

import (
	"errors"
	"fmt"
	"io"

	"github.com/arduino/go-paths-helper"
	"github.com/goccy/go-yaml"
)

type ModelDescriptor struct {
	ID          string         `yaml:"id"`
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Bricks      []BrickConfig  `yaml:"bricks"`
	Metadata    map[string]any `yaml:"metadata,omitempty"`
	// TODO: add more fields as needed
	// Runer
	// ModelLabel
}

type BrickConfig struct {
	ID                 string         `yaml:"id"`
	ModelConfiguration map[string]any `yaml:"model_configuration,omitempty"`
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
		// FIXME: probably we don't want to accept empty model.yaml files.
		if errors.Is(err, io.EOF) {
			return descriptor, nil
		}
		return ModelDescriptor{}, fmt.Errorf("cannot decode descriptor: %w", err)
	}
	return descriptor, nil
}

func (a *ModelDescriptor) IsValid() bool {
	/*  TODO: check
	1) brick list are present into the brick-list
	2) metadata are coherent with the source
	*/
	return true
}
