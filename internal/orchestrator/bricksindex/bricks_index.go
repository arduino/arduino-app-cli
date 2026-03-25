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
	"fmt"
	"io"
	"iter"
	"log/slog"
	"os"
	"slices"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/peripherals"
	"github.com/arduino/arduino-app-cli/internal/store"
)

type BricksIndex struct {
	YamlBricksIndex   *YamlBricksIndex
	folderBricksIndex *folderBricksIndex
}

type folderBricksIndex struct {
	path paths.PathList
}

type FolderBrick struct {
	FullPath *paths.Path

	composeFile  *paths.Path // brick_compose.yaml file path, optional
	readmeFile   *paths.Path // README.md file path, optional
	examplesPath *paths.Path // code examples folder path, optional
	docsAPIPath  *paths.Path // API docs file path, optional

	Brick Brick // the brick as defined in the brick_config.yaml file
}

func (f *folderBricksIndex) findBrick(id string) (*FolderBrick, bool) {
	for _, path := range f.path {
		brick, err := load(path)
		if err != nil {
			slog.Warn("Cannot load local app brick", "err", err, "path", path)
			continue
		}
		if brick.Brick.ID == id {
			return &brick, true
		}
	}
	return nil, false
}

func (f *folderBricksIndex) loadBricks() []Brick {
	bricks := []Brick{}
	for _, path := range f.path {
		brick, err := load(path)
		if err != nil {
			slog.Warn("Cannot load local app brick", "err", err, "path", path)
			continue
		}
		bricks = append(bricks, brick.Brick)
	}
	return bricks
}

func load(brickPath *paths.Path) (a FolderBrick, err error) {
	brickConfigPath := brickPath.Join("brick_config.yaml")
	if brickConfigPath.NotExist() {
		return FolderBrick{}, fmt.Errorf("brick_config.yaml does not exist: %v", brickConfigPath)
	}
	brickConfigContent, err := os.ReadFile(brickConfigPath.String())
	if err != nil {
		return FolderBrick{}, fmt.Errorf("cannot read brick_config.yaml: %w", err)
	}
	customBrick := Brick{}
	if err := yaml.Unmarshal(brickConfigContent, &customBrick); err != nil {
		return FolderBrick{}, fmt.Errorf("cannot unmarshal brick_config.yaml: %w", err)
	}
	customBrick.Source = "custom" // TODO: find a better name

	var composeFile *paths.Path = nil
	brickComposeFile := brickPath.Join("brick_compose.yaml")
	if brickComposeFile.Exist() {
		composeFile = brickComposeFile
	}

	return FolderBrick{
		FullPath:     brickPath,
		composeFile:  composeFile,
		readmeFile:   brickPath.Join("README.md"),
		examplesPath: brickPath.Join("examples"),
		docsAPIPath:  brickPath.Join("docs/API.md"),
		Brick:        customBrick,
	}, nil
}

func (m *BricksIndex) WithBricksFolder(path *paths.Path) *BricksIndex {
	if path == nil {
		return m
	}
	if !path.Exist() {
		slog.Warn("bricks folder path does not exist, skipping loading bricks from folder", "path", path)
		return m
	}
	pathsList, err := path.ReadDirRecursiveFiltered(func(file *paths.Path) bool {
		if file.Join("brick_config.yaml").NotExist() {
			// let's continue scanning, the model can be in a subfolder
			return true
		}
		return false
	}, paths.FilterDirectories(), paths.FilterOutNames(".cache"))
	if err != nil {
		slog.Warn("error reading bricks folder, skipping loading bricks from folder", "path", path, "err", err)
		return m
	}
	return &BricksIndex{YamlBricksIndex: m.YamlBricksIndex, folderBricksIndex: &folderBricksIndex{path: pathsList}}
}

func (b *BricksIndex) FindBrickByID(id string) (*Brick, bool) {
	brick, found := b.folderBricksIndex.findBrick(id)
	if found {
		return &brick.Brick, true
	}

	idx := slices.IndexFunc(b.YamlBricksIndex.Bricks, func(brick Brick) bool {
		return brick.ID == id
	})
	if idx == -1 {
		return nil, false
	}
	return &b.YamlBricksIndex.Bricks[idx], true
}

func (b *BricksIndex) ListBricks() []Brick {
	return append(b.YamlBricksIndex.Bricks, b.folderBricksIndex.loadBricks()...)
}

func (b *BricksIndex) GetReadme(id string) (string, error) {
	brick, found := b.folderBricksIndex.findBrick(id)
	if found {
		if brick.readmeFile.NotExist() {
			return "", fmt.Errorf("README.md not found for brick %s", id)
		}
		content, err := os.ReadFile(brick.readmeFile.String())
		if err != nil {
			return "", fmt.Errorf("cannot read README.md for brick %s: %w", id, err)
		}
		return string(content), nil
	}
	return b.YamlBricksIndex.store.GetBrickReadmeFromID(id)
}

func (b *BricksIndex) GetComposePath(id string) (*paths.Path, bool) {
	brick, found := b.folderBricksIndex.findBrick(id)
	if found {
		return brick.composeFile, brick.composeFile != nil
	}
	path, err := b.YamlBricksIndex.store.GetBrickComposeFilePathFromID(id)
	if err != nil {
		return nil, false
	}
	return path, true
}

func (b *BricksIndex) GetApiDocPath(id string) (*paths.Path, error) {
	brick, found := b.folderBricksIndex.findBrick(id)
	if found {
		if brick.docsAPIPath.NotExist() {
			return nil, fmt.Errorf("API.md not found for brick %s", id)
		}
		return brick.docsAPIPath, nil
	}
	p, err := b.YamlBricksIndex.store.GetBrickApiDocPathFromID(id)
	if err != nil {
		return nil, err
	}
	return paths.New(p), nil
}

func (b *BricksIndex) GetExamplesPath(id string) (paths.PathList, error) {
	brick, found := b.folderBricksIndex.findBrick(id)
	if found {
		if brick.examplesPath.NotExist() {
			return nil, fmt.Errorf("examples folder not found for brick %s", id)
		}
		dirEntries, err := brick.examplesPath.ReadDir()
		if err != nil {
			return nil, fmt.Errorf("cannot read examples directory for brick %s: %w", id, err)
		}
		return dirEntries, nil
	}
	return b.YamlBricksIndex.store.GetBrickCodeExamplesPathFromID(id)
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

	Source string `yaml:"-"` // Arduino or "other"

	composeFile  *paths.Path `yaml:"-"` // brick_compose.yaml file path, optional
	readmeFile   *paths.Path `yaml:"-"` // README.md file path, optional
	examplesPath *paths.Path `yaml:"-"` // code examples folder path, optional
	docsAPIPath  *paths.Path `yaml:"-"` // API docs file path, optional
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

func unmarshalBricksIndex(content io.Reader) (*YamlBricksIndex, error) {
	var index YamlBricksIndex
	if err := yaml.NewDecoder(content).Decode(&index); err != nil {
		return nil, err
	}
	return &index, nil
}

type YamlBricksIndex struct {
	Bricks []Brick `yaml:"bricks"`
	store  *store.StaticStore
}

func Load(store *store.StaticStore) (*BricksIndex, error) {
	content, err := store.GetAssetsFolder().Join("bricks-list.yaml").Open()
	if err != nil {
		return nil, err
	}
	defer content.Close()
	yamlIndex, err := unmarshalBricksIndex(content)
	if err != nil {
		return nil, err
	}
	for i := range yamlIndex.Bricks {
		yamlIndex.Bricks[i].Source = "Arduino"
		yamlIndex.Bricks[i].composeFile = nil
		yamlIndex.Bricks[i].readmeFile = nil
		yamlIndex.Bricks[i].examplesPath = nil
		yamlIndex.Bricks[i].docsAPIPath = nil
	}
	yamlIndex.store = store // needed to load example and readme files from the asset folder for built-in bricks
	return &BricksIndex{
		YamlBricksIndex: yamlIndex,
	}, nil
}
