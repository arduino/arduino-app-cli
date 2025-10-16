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

package app

import (
	"errors"
	"fmt"
	"io"

	emoji "github.com/Andrew-M-C/go.emoji"
	"github.com/arduino/go-paths-helper"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
)

type Brick struct {
	ID        string            `yaml:"-"` // Ignores this field, to be handled manually
	Model     string            `yaml:"model,omitempty"`
	Variables map[string]string `yaml:"variables,omitempty"`
}

type AppDescriptor struct {
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description"`
	Ports           []int    `yaml:"ports"`
	Bricks          []Brick  `yaml:"bricks"`
	Icon            string   `yaml:"icon,omitempty"`
	RequiredDevices []string `yaml:"required_devices,omitempty"`
}

func (d AppDescriptor) MarshalYAML() (any, error) {
	type raw struct {
		Name            string             `yaml:"name"`
		Description     string             `yaml:"description"`
		Ports           []int              `yaml:"ports"`
		Bricks          []map[string]Brick `yaml:"bricks"`
		Icon            string             `yaml:"icon,omitempty"`
		RequiredDevices []string           `yaml:"required_devices,omitempty"`
	}

	bricks := make([]map[string]Brick, len(d.Bricks))
	for i, brick := range d.Bricks {
		bricks[i] = map[string]Brick{brick.ID: brick}
	}
	return &raw{
		Name:            d.Name,
		Description:     d.Description,
		Ports:           d.Ports,
		Bricks:          bricks,
		Icon:            d.Icon,
		RequiredDevices: d.RequiredDevices,
	}, nil
}

func (md *Brick) UnmarshalYAML(node ast.Node) error {
	switch node.Type() {
	case ast.StringType: // String type brick (i.e. "- arduino:brickname").
		md.ID = node.(*ast.StringNode).Value
	case ast.MappingType: // Map type brick (name followed by a ':' and, optionally, some fields).
		content := node.(*ast.MappingNode).Values
		if len(content) == 0 {
			return fmt.Errorf("expected single-key map for brick item")
		}

		keyNode := content[0].Key
		valueNode := content[0].Value

		switch valueNode.Type() {
		case ast.NullType:
		case ast.MappingType:
			// This alias is used to bypass the custom UnmarshalYAML when decoding the inner details map.
			type brickAlias Brick
			var details brickAlias

			if err := yaml.Unmarshal([]byte(valueNode.String()), &details); err != nil {
				return fmt.Errorf("failed to unmarshal brick details for '%s': %w", md.ID, err)
			}
			*md = Brick(details)
		default:
			return fmt.Errorf("unexpected value type for brick key '%s' (expected map or null, got %v)",
				valueNode.String(), keyNode.String())
		}
		if keyNode.Type() == ast.StringType {
			md.ID = keyNode.(*ast.StringNode).Value
		}

	default:
		// The node is neither a scalar string nor a map.
		return fmt.Errorf("expected scalar or mapping node for dependency item, got %v", node.String())
	}

	return nil
}

// ParseAppFile reads an app file
func ParseDescriptorFile(file *paths.Path) (AppDescriptor, error) {
	f, err := file.Open()
	if err != nil {
		return AppDescriptor{}, fmt.Errorf("cannot open file: %w", err)
	}
	defer f.Close()
	descriptor := AppDescriptor{}
	if err := yaml.NewDecoder(f).Decode(&descriptor); err != nil {
		// FIXME: probably we don't want to accept empty app.yaml files.
		if errors.Is(err, io.EOF) {
			return descriptor, nil
		}
		return AppDescriptor{}, fmt.Errorf("cannot decode descriptor: %w", err)
	}

	if descriptor.Name == "" {
		return AppDescriptor{}, fmt.Errorf("application name is empty")
	}

	return descriptor, descriptor.IsValid()
}

func (a *AppDescriptor) IsValid() error {
	var allErrors error
	if a.Icon != "" {
		if !isSingleEmoji(a.Icon) {
			allErrors = errors.Join(allErrors, fmt.Errorf("icon %q is not a valid single emoji", a.Icon))
		}
	}
	return allErrors
}

func isSingleEmoji(s string) bool {
	emojis := 0
	for it := emoji.IterateChars(s); it.Next(); {
		if !it.CurrentIsEmoji() {
			return false
		}
		// Skip variation selectors (0xFE00-0xFE0F)
		if it.Current() >= "\uFE00" && it.Current() <= "\uFE0F" {
			continue
		}
		emojis++
	}
	return emojis == 1
}
