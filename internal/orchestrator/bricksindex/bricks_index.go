// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	yaml "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/peripherals"
	"github.com/arduino/arduino-app-cli/internal/platform"
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

type ModelsBoard struct {
	Platform string `yaml:"platform"`
	Model    string `yaml:"model"`
}

func (v BrickVariable) IsRequired() bool {
	return v.DefaultValue == ""
}

type RequiresServices []RequiresService

type RequiresService struct {
	ID   string                `yaml:"id"`
	When *RequiresServiceMatch `yaml:"when,omitempty"`
}

type RequiresServiceMatch struct {
	Model *string `yaml:"model,omitempty"`
}

func (r *RequiresServices) UnmarshalYAML(node ast.Node) error {
	seq, ok := node.(*ast.SequenceNode)
	if !ok {
		return fmt.Errorf("requires_services: expected a sequence, got %s", node.Type())
	}
	*r = make(RequiresServices, 0, len(seq.Values))
	for _, item := range seq.Values {
		switch item.Type() {
		case ast.StringType:
			// Plain string form: "- arduino:genie_audio"
			*r = append(*r, RequiresService{ID: item.(*ast.StringNode).Value})
		case ast.MappingType:
			// Struct form: "- id: arduino:genie\n  when:\n    model: genie:*"
			var svc RequiresService
			if err := yaml.Unmarshal([]byte(item.String()), &svc); err != nil {
				return fmt.Errorf("requires_services: failed to unmarshal service entry: %w", err)
			}
			*r = append(*r, svc)
		default:
			return fmt.Errorf("requires_services: unexpected node type %s", item.Type())
		}
	}
	return nil
}

type Brick struct {
	ID              string   `yaml:"id"`
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description"`
	SupportedBoards []string `yaml:"supported_boards,omitempty"`
	Category        string   `yaml:"category,omitempty"`
	RequiresDisplay string   `yaml:"requires_display,omitempty"`
	// Deprecated : the field `require_container` is deprecated, you can remove it from the brick config. It will be ignored if present.
	RequireContainer            bool                      `yaml:"require_container"` // Deprecated
	Variables                   []BrickVariable           `yaml:"variables,omitempty"`
	Ports                       []string                  `yaml:"ports,omitempty"`
	ModelName                   string                    `yaml:"model_name,omitempty"`
	ModelByBoard                []ModelsBoard             `yaml:"model_by_boards,omitempty"`
	MountDevicesIntoContainer   bool                      `yaml:"mount_devices_into_container,omitempty"`
	RequiredDevices             []peripherals.DeviceClass `yaml:"required_devices,omitempty"`
	RequiresServices            RequiresServices          `yaml:"requires_services,omitempty"`
	ModelConfigurationVariables []string                  `yaml:"model_configuration_variables,omitempty"`

	Source string `yaml:"-"`

	FullPath     *paths.Path `yaml:"-"`
	ComposeFile  *paths.Path `yaml:"-"` // brick_compose.yaml file path, optional
	ReadmeFile   *paths.Path `yaml:"-"` // README.md file path, optional
	ExamplesPath *paths.Path `yaml:"-"` // code examples folder path, optional
	DocsAPIPath  *paths.Path `yaml:"-"` // API docs file path, optional
	RequireModel bool        `yaml:"-"`

	containerPorts []string `yaml:"-"` // Ports extracted from the compose file, optional
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

func (b Brick) GetPorts() []string {
	ports := make([]string, 0, len(b.Ports)+len(b.containerPorts))
	ports = append(ports, b.Ports...)
	ports = append(ports, b.containerPorts...)
	slices.Sort(ports)
	return slices.Compact(ports)
}

func (b Brick) isAiBrick(platform platform.Platform) bool {
	for _, mb := range b.ModelByBoard {
		if mb.Platform == platform.BoardName && mb.Model != "" {
			return true
		}
	}
	return b.ModelName != ""
}

type BrickInstance struct {
	Model string
}

// GetMatchingService returns the list of service IDs required by the brick for the given instance.
// It evaluates any conditional constraints (e.g. model pattern matching) defined in each RequiresService entry.
//
// A service with no "when" condition is always included. A service with "when.model" is included
// only when the instance model matches the pattern (e.g. "genie:*" matches "genie:mini", "genie:pro").
func (b Brick) GetMatchingService(brick BrickInstance) ([]string, error) {
	services := make([]string, 0, len(b.RequiresServices))
	for _, r := range b.RequiresServices {
		if r.When == nil {
			services = append(services, r.ID)
			continue
		}
		if r.When.Model == nil {
			services = append(services, r.ID)
			continue
		}
		if ok, err := path.Match(*r.When.Model, brick.Model); err != nil {
			return services, fmt.Errorf("invalid pattern in requires_services.when.model: %w", err)
		} else if ok {
			services = append(services, r.ID)
		}
	}
	return services, nil
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

func Load(platform platform.Platform, path *paths.Path) (*BricksIndex, error) {
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
		if yamlIndex.Bricks[i].RequireContainer {
			slog.Warn("the field `require_container` is deprecated. You can remove it from the brick config", "brick_id", yamlIndex.Bricks[i].ID)
		}
		yamlIndex.Bricks[i].Source = "Arduino"
		yamlIndex.Bricks[i].FullPath = path
		yamlIndex.Bricks[i].ReadmeFile = path.Join("docs", namespace, brickName, "README.md")
		yamlIndex.Bricks[i].ExamplesPath = path.Join("examples", namespace, brickName)
		yamlIndex.Bricks[i].DocsAPIPath = path.Join("api-docs", namespace, "app_bricks", brickName, "API.md")
		yamlIndex.Bricks[i].RequireModel = yamlIndex.Bricks[i].isAiBrick(platform)

		// Load main compose file and, if present, platform-specific compose files
		var (
			composePath     = path.Join("compose", namespace, brickName)
			baseCompose     = composePath.Join("brick_compose.yaml")
			specificCompose = composePath.Join(fmt.Sprintf("brick_compose.%s.yaml", platform.BoardName))
		)
		if platform.BoardName != "" && specificCompose.Exist() {
			yamlIndex.Bricks[i].ComposeFile = specificCompose
		} else if baseCompose.Exist() {
			yamlIndex.Bricks[i].ComposeFile = baseCompose
		}

		// Extract ports from the compose file if it exists
		if composeFile, ok := yamlIndex.Bricks[i].GetComposeFile(); ok {
			if ports, err := extractPortsFromComposeFile(composeFile); err == nil {
				yamlIndex.Bricks[i].containerPorts = ports
			} else {
				slog.Warn("cannot extract ports from compose file, skipping", "brick_id", yamlIndex.Bricks[i].ID, "error", err)
			}
		}

		// Resolve the board-specific model name, if any.
		if platform.BoardName != "" {
			idx := slices.IndexFunc(yamlIndex.Bricks[i].ModelByBoard, func(mb ModelsBoard) bool {
				return mb.Platform == platform.BoardName
			})
			if idx != -1 {
				yamlIndex.Bricks[i].ModelName = yamlIndex.Bricks[i].ModelByBoard[idx].Model
			}
		}
	}

	yamlIndex.Bricks = slices.DeleteFunc(yamlIndex.Bricks, func(brick Brick) bool {
		return platform.BoardName != "" &&
			len(brick.SupportedBoards) != 0 &&
			!slices.Contains(brick.SupportedBoards, platform.BoardName)
	})

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

func extractPortsFromComposeFile(composeFile *paths.Path) ([]string, error) {
	content, err := composeFile.ReadFile()
	if err != nil {
		return nil, err
	}

	prj, err := loader.LoadWithContext(
		context.Background(),
		types.ConfigDetails{
			ConfigFiles: []types.ConfigFile{{Content: content}},
			Environment: types.NewMapping(os.Environ()),
		},
		func(o *loader.Options) { o.SetProjectName("default", false); o.SkipConsistencyCheck = true },
		loader.WithSkipValidation,
	)
	if err != nil {
		return nil, err
	}

	var ports []string
	for _, svc := range prj.Services {
		for _, p := range svc.Ports {
			if p.Published != "" {
				ports = append(ports, p.Published)
			} else {
				ports = append(ports, fmt.Sprintf("%d", p.Target))
			}
		}
	}
	return ports, nil
}
