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

package bricksindex

import (
	"io"
	"iter"
	"slices"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"
)

type BricksIndex struct {
	Bricks []Brick `yaml:"bricks"`
}

func (b *BricksIndex) FindBrickByID(id string) (*Brick, bool) {
	idx := slices.IndexFunc(b.Bricks, func(brick Brick) bool {
		return brick.ID == id
	})
	if idx == -1 {
		return nil, false
	}
	return &b.Bricks[idx], true
}

type BrickVariable struct {
	Name         string `yaml:"name"`
	DefaultValue string `yaml:"default_value"`
	Description  string `yaml:"description,omitempty"`
}

func (v BrickVariable) IsRequired() bool {
	return v.DefaultValue == ""
}

type Brick struct {
	ID                        string          `yaml:"id"`
	Name                      string          `yaml:"name"`
	Description               string          `yaml:"description"`
	Category                  string          `yaml:"category,omitempty"`
	RequiresDisplay           string          `yaml:"requires_display,omitempty"`
	RequireContainer          bool            `yaml:"require_container"`
	RequireModel              bool            `yaml:"require_model"`
	Variables                 []BrickVariable `yaml:"variables,omitempty"`
	Ports                     []string        `yaml:"ports,omitempty"`
	ModelName                 string          `yaml:"model_name,omitempty"`
	MountDevicesIntoContainer bool            `yaml:"mount_devices_into_container,omitempty"`
	RequiredDevices           []string        `yaml:"required_devices,omitempty"`
}

func (b Brick) GetVariable(name string) (BrickVariable, bool) {
	idx := slices.IndexFunc(b.Variables, func(variable BrickVariable) bool {
		return variable.Name == name
	})
	if idx == -1 {
		return BrickVariable{}, false
	}
	return b.Variables[idx], true
}

func (b Brick) GetDefaultVariables() iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		for _, v := range b.Variables {
			if !yield(v.Name, v.DefaultValue) {
				return
			}
		}
	}
}

func unmarshalBricksIndex(content io.Reader) (*BricksIndex, error) {
	var index BricksIndex
	if err := yaml.NewDecoder(content).Decode(&index); err != nil {
		return nil, err
	}
	return &index, nil
}

func GenerateBricksIndexFromFile(dir *paths.Path) (*BricksIndex, error) {
	content, err := dir.Join("bricks-list.yaml").Open()
	if err != nil {
		return nil, err
	}
	defer content.Close()
	return unmarshalBricksIndex(content)
}
