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
	"errors"
	"fmt"
	"iter"
	"os"
	"slices"
	"strings"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/peripherals"
)

type BricksIndex struct {
	Bricks []Brick `yaml:"bricks"`

	AssetPath *paths.Path `yaml:"-"`
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

func (l *BricksIndex) ListBricks() ([]Brick, bool) {
	return l.Bricks, true
}

func (l *BricksIndex) ComposePath(id string) (*paths.Path, bool) {
	namespace, brickName, err := parseBrickID(id)
	if err != nil {
		return nil, false
	}
	return l.AssetPath.Join("compose", namespace, brickName, "brick_compose.yaml"), true
}

func (s *BricksIndex) GetBrickApiDocPathFromID(brickID string) (string, error) {
	namespace, brickName, err := parseBrickID(brickID)
	if err != nil {
		return "", err
	}
	return s.AssetPath.Join("api-docs", namespace, "app_bricks", brickName, "API.md").String(), nil
}

func (s *BricksIndex) GetBrickReadmeFromID(brickID string) (string, error) {
	namespace, brickName, err := parseBrickID(brickID)
	if err != nil {
		return "", err
	}
	return s.AssetPath.Join("docs", namespace, brickName, "README.md").String(), nil
}

func (s *BricksIndex) GetBrickCodeExamplesPathFromID(brickID string) (paths.PathList, error) {
	namespace, brickName, err := parseBrickID(brickID)
	if err != nil {
		return nil, err
	}
	targetDir := s.AssetPath.Join("code-examples", namespace, brickName)
	dirEntries, err := targetDir.ReadDir()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read examples directory %q: %w", targetDir, err)
	}
	return dirEntries, nil
}

func parseBrickID(brickID string) (namespace, name string, err error) {
	namespace, brickName, ok := strings.Cut(brickID, ":")
	if !ok {
		return "", "", errors.New("invalid ID")
	}
	return namespace, brickName, nil
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

func Load(dir *paths.Path) (*BricksIndex, error) {
	content, err := dir.Join("bricks-list.yaml").Open()
	if err != nil {
		return nil, err
	}
	defer content.Close()
	var index BricksIndex
	if err := yaml.NewDecoder(content).Decode(&index); err != nil {
		return nil, err
	}
	index.AssetPath = dir
	return &index, nil
}
