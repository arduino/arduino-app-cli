package aimodel

import (
	"errors"
	"fmt"
	"io"

	"github.com/arduino/go-paths-helper"
	"github.com/goccy/go-yaml"
)

type ModelDescriptor struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Bricks      []string `yaml:"bricks"`
	// ModelLabels []string
	// ModelConfiguration THIS MUST BE REMOVED IN FAVOR OF A LIST OF BRICK WITH MODEL CONFIGURATION

	// TODO: put into a metadata field ??
	// Source    string `yaml:source`
	// Category  string `yaml:"category"`
	// ProjectID int    `yaml:"project-id"`
	// ImpulseID int    `yaml:"impulse-id"`
	// LastBuildAt time.Time `yaml:"lastBuildAt" json:"lastBuildAt"`
}

// ParseAppFile reads an app file
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
	return descriptor, descriptor.IsValid()
}

func (a *ModelDescriptor) IsValid() error {
	/*  TODO: check
	1) brick list are present into the brick-list
	2) metadata are coherent with the source
	*/
	return nil
}
