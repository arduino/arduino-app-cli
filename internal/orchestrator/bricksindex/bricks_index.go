package bricksindex

import (
	"path"
	"slices"

	yaml "github.com/goccy/go-yaml"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/assets"
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
	RequireContainer bool            `yaml:"require_container"`
	RequireModel     bool            `yaml:"require_model"`
	Variables        []BrickVariable `yaml:"variables,omitempty"`
	Ports            []string        `yaml:"ports,omitempty"`
	ModelName        string          `yaml:"model_name,omitempty"`
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

func GenerateBricksIndex() (*BricksIndex, error) {
	versions, err := assets.FS.ReadDir("static")
	if err != nil {
		return nil, err
	}
	f.Assert(len(versions) == 1, "No bricks available in the assets directory")

	bricksList, err := assets.FS.ReadFile(path.Join("static", versions[0].Name(), "bricks-list.yaml"))
	if err != nil {
		return nil, err
	}

	var index BricksIndex
	if err := yaml.Unmarshal(bricksList, &index); err != nil {
		return nil, err
	}
	return &index, nil
}
