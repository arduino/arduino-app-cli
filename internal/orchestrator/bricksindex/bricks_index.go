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
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/peripherals"
)

type BricksIndex struct {
	BuiltInBricks []Brick
	bricksFolders paths.PathList
}

func (m *BricksIndex) WithAppBricks(app *app.ArduinoApp) *BricksIndex {
	if app == nil {
		return m
	}
	bricksDir := app.GetBricksPath()
	if !bricksDir.Exist() {
		return m
	}
	pathsList, err := bricksDir.ReadDirRecursiveFiltered(func(file *paths.Path) bool {
		if file.Join("brick_config.yaml").NotExist() {
			// let's continue scanning, the model can be in a subfolder
			return true
		}
		return false
	}, paths.FilterDirectories(), paths.FilterOutNames(".cache"))
	if err != nil {
		slog.Warn("error reading app bricks folder, skipping loading bricks from app", "app", app.Name, "err", err)
		return m
	}
	return &BricksIndex{BuiltInBricks: m.BuiltInBricks, bricksFolders: pathsList}
}

func (b *BricksIndex) FindBrickByID(id string) (*Brick, bool) {
	bricks := b.ListBricks()
	idx := slices.IndexFunc(bricks, func(brick Brick) bool {
		return brick.ID == id
	})
	if idx == -1 {
		return nil, false
	}
	return &bricks[idx], true
}

func (b *BricksIndex) ListBricks() []Brick {
	return append(b.BuiltInBricks, b.loadBricksFromFolders()...)
}

func (f *BricksIndex) loadBricksFromFolders() []Brick {
	bricks := []Brick{}
	for _, path := range f.bricksFolders {
		brick, err := load(path)
		if err != nil {
			slog.Warn("Cannot load local app brick", "err", err, "path", path)
			continue
		}
		bricks = append(bricks, brick)
	}
	return bricks
}

func load(brickPath *paths.Path) (a Brick, err error) {
	brickConfigPath := brickPath.Join("brick_config.yaml")
	if brickConfigPath.NotExist() {
		return Brick{}, fmt.Errorf("brick_config.yaml does not exist: %v", brickConfigPath)
	}
	brickConfigContent, err := os.ReadFile(brickConfigPath.String())
	if err != nil {
		return Brick{}, fmt.Errorf("cannot read brick_config.yaml: %w", err)
	}
	brick := Brick{}
	if err := yaml.Unmarshal(brickConfigContent, &brick); err != nil {
		return Brick{}, fmt.Errorf("cannot unmarshal brick_config.yaml: %w", err)
	}
	brick.Source = "custom" // TODO: find a better name

	var composeFile *paths.Path = nil
	brickComposeFile := brickPath.Join("brick_compose.yaml")
	if brickComposeFile.Exist() {
		composeFile = brickComposeFile
	}
	brick.Source = "custom" // TODO: find a better name ?
	brick.composeFile = composeFile
	brick.readmeFile = brickPath.Join("README.md")
	brick.examplesPath = brickPath.Join("examples")
	brick.docsAPIPath = brickPath.Join("docs/API.md")
	return brick, nil
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

	Source string `yaml:"-"`

	composeFile  *paths.Path `yaml:"-"` // brick_compose.yaml file path, optional
	readmeFile   *paths.Path `yaml:"-"` // README.md file path, optional
	examplesPath *paths.Path `yaml:"-"` // code examples folder path, optional
	docsAPIPath  *paths.Path `yaml:"-"` // API docs file path, optional
}

func (b Brick) GetComposeFile() (*paths.Path, bool) {
	if b.composeFile == nil || b.composeFile.NotExist() {
		return nil, false
	}
	return b.composeFile, true
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
	if b.readmeFile == nil || b.readmeFile.NotExist() {
		return "", fmt.Errorf("README.md not found for brick %s", b.ID)
	}
	content, err := os.ReadFile(b.readmeFile.String())
	if err != nil {
		return "", fmt.Errorf("cannot read README.md for brick %s: %w", b.ID, err)
	}
	return string(content), nil
}

func (b Brick) GetExamplesPath() (paths.PathList, error) {
	if b.examplesPath == nil || b.examplesPath.NotExist() {
		return nil, fmt.Errorf("examples not found for brick %s", b.ID)
	}
	dirEntries, err := b.examplesPath.ReadDir()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("examples not found for brick %s", b.ID)
		}
		return nil, fmt.Errorf("cannot read examples directory %q: %w", b.examplesPath, err)
	}
	return dirEntries, nil
}

func (b Brick) GetApiDocPath() (*paths.Path, bool) {
	if b.docsAPIPath == nil || b.docsAPIPath.NotExist() {
		return nil, false
	}
	return b.docsAPIPath, true
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
	bricks, err := unmarshalBricksIndex(content)
	if err != nil {
		return nil, err
	}
	for i := range bricks.Bricks {
		namespace, brickName, err := parseBrickID(bricks.Bricks[i].ID)
		if err != nil {
			return nil, err
		}
		bricks.Bricks[i].Source = "Arduino"
		bricks.Bricks[i].composeFile = path.Join("compose", namespace, brickName, "brick_compose.yaml")
		bricks.Bricks[i].readmeFile = path.Join("docs", namespace, brickName, "README.md")
		bricks.Bricks[i].examplesPath = path.Join("examples", namespace, brickName)
		bricks.Bricks[i].docsAPIPath = path.Join("api-docs", namespace, "app_bricks", brickName, "API.md")
	}
	return &BricksIndex{
		BuiltInBricks: bricks.Bricks,
	}, nil
}

func parseBrickID(brickID string) (namespace, name string, err error) {
	namespace, brickName, ok := strings.Cut(brickID, ":")
	if !ok {
		return "", "", errors.New("invalid ID")
	}
	return namespace, brickName, nil
}
