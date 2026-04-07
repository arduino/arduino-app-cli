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

package custommodel

import (
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

func (a *ModelDescriptor) IsValid() bool {
	/*  TODO: check
	1) brick list are present into the brick-list
	2) metadata are coherent with the source
	*/
	return true
}
