// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"errors"
	"fmt"
	"io"
	"regexp"

	emoji "github.com/Andrew-M-C/go.emoji"
	"github.com/arduino/go-paths-helper"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	semver "go.bug.st/relaxed-semver"
)

type Brick struct {
	ID        string            `yaml:"-"` // Ignores this field, to be handled manually
	Version   string            `yaml:"-"`
	Model     string            `yaml:"model,omitempty"`
	Variables map[string]string `yaml:"variables,omitempty"`
	Devices   []string          `yaml:"devices,omitempty"`
}

type AppDescriptor struct {
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	Ports       []int   `yaml:"ports"`
	Bricks      []Brick `yaml:"bricks"`
	Icon        string  `yaml:"icon,omitempty"`
	// Deprecated: Use the RequiredDevices section defined per Brick instead.
	RequiredDevices []string `yaml:"required_devices,omitempty"`
}

//nolint:unparam // yaml.Marshaler pattern requires an error return even when this implementation cannot fail.
func (d AppDescriptor) MarshalYAML() (any, error) {
	type raw struct {
		Name        string             `yaml:"name"`
		Description string             `yaml:"description"`
		Ports       []int              `yaml:"ports"`
		Bricks      []map[string]Brick `yaml:"bricks"`
		Icon        string             `yaml:"icon,omitempty"`
		// Deprecated: Use the RequiredDevices section defined per Brick instead.
		RequiredDevices []string `yaml:"required_devices,omitempty"`
	}

	bricks := make([]map[string]Brick, len(d.Bricks))
	for i, brick := range d.Bricks {
		bricks[i] = map[string]Brick{brick.Reference(): brick}
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

var pinnedBrickRegexp = regexp.MustCompile(`^(.+) \((.*)\)$`)

// parseBrickReference splits a brick reference of the app.yaml, in the form "ID" or
// "ID (VERSION)", into the id and the version, which is empty when not pinned.
func parseBrickReference(ref string) (id, version string, err error) {
	match := pinnedBrickRegexp.FindStringSubmatch(ref)
	if match == nil {
		return ref, "", nil
	}
	id, version = match[1], match[2]
	if _, err := semver.Parse(version); version == "" || err != nil {
		return "", "", fmt.Errorf("invalid version %q for brick %q", version, id)
	}
	return id, version, nil
}

// Reference is how the brick is written in the app.yaml: the id, followed by the version
// in parentheses when it is pinned.
func (md Brick) Reference() string {
	if md.Version == "" {
		return md.ID
	}
	return md.ID + " (" + md.Version + ")"
}

func (md *Brick) UnmarshalYAML(node ast.Node) error {
	var ref string
	switch node.Type() {
	case ast.StringType: // String type brick (i.e. "- arduino:brickname").
		ref = node.(*ast.StringNode).Value
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
			ref = keyNode.(*ast.StringNode).Value
		}

	default:
		// The node is neither a scalar string nor a map.
		return fmt.Errorf("expected scalar or mapping node for dependency item, got %v", node.String())
	}

	id, version, err := parseBrickReference(ref)
	if err != nil {
		return err
	}
	md.ID, md.Version = id, version
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
