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

package bricksindex

import (
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"slices"
	"strings"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/peripherals"
)

type BricksIndex struct {
	BuiltInBricks []Brick
	AppBricks     []Brick
}

func (b *BricksIndex) WithAppBricks(bricks []Brick) *BricksIndex {
	b.AppBricks = bricks
	return b
}

func (b *BricksIndex) FindBrickByID(id string) (*Brick, bool) {
	searchFunc := func(brick Brick) bool {
		return brick.ID == id
	}
	if idx := slices.IndexFunc(b.AppBricks, searchFunc); idx != -1 {
		return &b.AppBricks[idx], true
	}
	if idx := slices.IndexFunc(b.BuiltInBricks, searchFunc); idx != -1 {
		return &b.BuiltInBricks[idx], true
	}
	return nil, false
}

// TODO: use iterator instead of returning a slice
func (b *BricksIndex) ListBricks() []Brick {
	bricks := slices.Concat(b.AppBricks, b.BuiltInBricks)
	slices.SortFunc(bricks, func(a, b Brick) int {
		return strings.Compare(a.Name, b.Name)
	})
	return bricks
}

type BrickVariable struct {
	Name         string `yaml:"name"`
	DefaultValue string `yaml:"default_value"`
	Description  string `yaml:"description,omitempty"`
	Hidden       bool   `yaml:"hidden"`
	Secret       bool   `yaml:"secret"`
}

func (v BrickVariable) IsRequired() bool {
	return v.DefaultValue == ""
}

type Brick struct {
	ID                        string                    `yaml:"id"`
	Name                      string                    `yaml:"name"`
	Description               string                    `yaml:"description"`
	Category                  string                    `yaml:"category,omitempty"`
	RequiresDisplay           string                    `yaml:"requires_display,omitempty"`
	RequireContainer          bool                      `yaml:"require_container"`
	RequireModel              bool                      `yaml:"require_model"`
	Variables                 []BrickVariable           `yaml:"variables,omitempty"`
	Ports                     []string                  `yaml:"ports,omitempty"`
	ModelName                 string                    `yaml:"model_name,omitempty"`
	MountDevicesIntoContainer bool                      `yaml:"mount_devices_into_container,omitempty"`
	RequiredDevices           []peripherals.DeviceClass `yaml:"required_devices,omitempty"`
	RequiresServices          []string                  `yaml:"requires_services,omitempty"`

	Source string `yaml:"-"`

	FullPath     *paths.Path `yaml:"-"`
	ComposeFile  *paths.Path `yaml:"-"` // brick_compose.yaml file path, optional
	ReadmeFile   *paths.Path `yaml:"-"` // README.md file path, optional
	ExamplesPath *paths.Path `yaml:"-"` // code examples folder path, optional
	DocsAPIPath  *paths.Path `yaml:"-"` // API docs file path, optional
}

func (b Brick) GetComposeFile() (*paths.Path, bool) {
	if b.ComposeFile == nil || b.ComposeFile.NotExist() {
		return nil, false
	}
	return b.ComposeFile, true
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

func (b Brick) GetReadmeFile() (string, error) {
	if b.ReadmeFile == nil || b.ReadmeFile.NotExist() {
		return "", fmt.Errorf("README.md not found for brick %s", b.ID)
	}
	content, err := os.ReadFile(b.ReadmeFile.String())
	if err != nil {
		return "", fmt.Errorf("cannot read README.md for brick %s: %w", b.ID, err)
	}
	return string(content), nil
}

func (b Brick) GetExamplesPath() (paths.PathList, error) {
	if b.ExamplesPath == nil || b.ExamplesPath.NotExist() {
		return nil, fmt.Errorf("examples not found for brick %s", b.ID)
	}
	dirEntries, err := b.ExamplesPath.ReadDir()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("examples not found for brick %s", b.ID)
		}
		return nil, fmt.Errorf("cannot read examples directory %q: %w", b.ExamplesPath, err)
	}
	return dirEntries, nil
}

func (b Brick) GetApiDocPath() (*paths.Path, bool) {
	if b.DocsAPIPath == nil || b.DocsAPIPath.NotExist() {
		return nil, false
	}
	return b.DocsAPIPath, true
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

type YamlBricksIndex struct {
	Bricks []Brick `yaml:"bricks"`
}

func unmarshalBricksIndex(content io.Reader) (*YamlBricksIndex, error) {
	var index YamlBricksIndex
	if err := yaml.NewDecoder(content).Decode(&index); err != nil {
		return nil, err
	}
	return &index, nil
}

func Load(path *paths.Path) (*BricksIndex, error) {
	content, err := path.Join("bricks-list.yaml").Open()
	if err != nil {
		return nil, err
	}
	defer content.Close()
	yamlIndex, err := unmarshalBricksIndex(content)
	if err != nil {
		return nil, err
	}
	for i := range yamlIndex.Bricks {
		namespace, brickName, err := parseBrickID(yamlIndex.Bricks[i].ID)
		if err != nil {
			return nil, err
		}
		yamlIndex.Bricks[i].Source = "Arduino"
		yamlIndex.Bricks[i].FullPath = path
		yamlIndex.Bricks[i].ComposeFile = path.Join("compose", namespace, brickName, "brick_compose.yaml")
		yamlIndex.Bricks[i].ReadmeFile = path.Join("docs", namespace, brickName, "README.md")
		yamlIndex.Bricks[i].ExamplesPath = path.Join("examples", namespace, brickName)
		yamlIndex.Bricks[i].DocsAPIPath = path.Join("api-docs", namespace, "app_bricks", brickName, "API.md")
	}
	return &BricksIndex{
		BuiltInBricks: yamlIndex.Bricks,
	}, nil
}

func parseBrickID(brickID string) (namespace, name string, err error) {
	namespace, brickName, ok := strings.Cut(brickID, ":")
	if !ok {
		return "", "", errors.New("invalid ID")
	}
	return namespace, brickName, nil
}
