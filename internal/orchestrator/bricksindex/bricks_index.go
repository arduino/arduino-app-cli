package bricksindex

import (
	"io"
	"io/fs"
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
	ID               string          `yaml:"id"`
	Name             string          `yaml:"name"`
	Description      string          `yaml:"description"`
	Category         string          `yaml:"category,omitempty"`
	RequireContainer bool            `yaml:"require_container"`
	RequireModel     bool            `yaml:"require_model"`
	Variables        []BrickVariable `yaml:"variables,omitempty"`
	Ports            []string        `yaml:"ports,omitempty"`
	ModelName        string          `yaml:"model_name,omitempty"`
	RequiresDevices  bool            `yaml:"require_devices,omitempty"`
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

func GenerateBricksIndex(fs fs.FS) (*BricksIndex, error) {
	file, err := fs.Open("bricks-list.yaml")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return unmarshalBricksIndex(content)
}

func unmarshalBricksIndex(bricksList []byte) (*BricksIndex, error) {
	var index BricksIndex
	if err := yaml.Unmarshal(bricksList, &index); err != nil {
		return nil, err
	}
	return &index, nil
}

func LoadBricksIndex(bricksList []byte) (*BricksIndex, error) {
	return unmarshalBricksIndex(bricksList)
}

func GenerateBricksIndexFromFile(dir *paths.Path) (*BricksIndex, error) {
	content, err := dir.Join("bricks-list.yaml").ReadFile()
	if err != nil {
		return nil, err
	}
	return unmarshalBricksIndex(content)
}
