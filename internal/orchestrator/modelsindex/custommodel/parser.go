// This file is part of arduino-app-cli.
//
// Copyright Copyright (C) Arduino s.r.l. and/or its affiliated companies
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
