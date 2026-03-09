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

package builtin

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

type BuiltinBricksIndex struct {
	Bricks []bricksindex.Brick `yaml:"bricks"`

	AssetPath *paths.Path `yaml:"-"`
}

func Load(dir *paths.Path) (*BuiltinBricksIndex, error) {
	content, err := dir.Join("bricks-list.yaml").Open()
	if err != nil {
		return nil, err
	}
	defer content.Close()
	var index BuiltinBricksIndex
	if err := yaml.NewDecoder(content).Decode(&index); err != nil {
		return nil, err
	}
	index.AssetPath = dir
	return &index, nil
}

func (b *BuiltinBricksIndex) GetByID(id string) (*bricksindex.Brick, bool) {
	idx := slices.IndexFunc(b.Bricks, func(brick bricksindex.Brick) bool {
		return brick.ID == id
	})
	if idx == -1 {
		return nil, false
	}
	return &b.Bricks[idx], true
}

func (l *BuiltinBricksIndex) ListBricks() ([]bricksindex.Brick, bool) {
	return l.Bricks, true
}

func (l *BuiltinBricksIndex) GetComposePath(id string) (*paths.Path, bool) {
	namespace, brickName, err := parseBrickID(id)
	if err != nil {
		return nil, false
	}
	return l.AssetPath.Join("compose", namespace, brickName, "brick_compose.yaml"), true
}

func (s *BuiltinBricksIndex) GetApiDocPath(brickID string) (*paths.Path, error) {
	namespace, brickName, err := parseBrickID(brickID)
	if err != nil {
		return nil, err
	}
	return s.AssetPath.Join("api-docs", namespace, "app_bricks", brickName, "API.md"), nil
}

func (s *BuiltinBricksIndex) GetReadme(brickID string) (string, error) {
	namespace, brickName, err := parseBrickID(brickID)
	if err != nil {
		return "", err
	}
	readmePath := s.AssetPath.Join("docs", namespace, brickName, "README.md")
	content, err := os.ReadFile(readmePath.String())
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (s *BuiltinBricksIndex) GetCodeExamplesPath(brickID string) (paths.PathList, error) {
	namespace, brickName, err := parseBrickID(brickID)
	if err != nil {
		return nil, err
	}
	codeExamplesPath := s.AssetPath.Join("examples", namespace, brickName)
	dirEntries, err := codeExamplesPath.ReadDir()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read examples directory %q: %w", codeExamplesPath, err)
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
