package parser

import (
	"errors"
	"fmt"
	"io"

	emoji "github.com/Andrew-M-C/go.emoji"
	"github.com/arduino/go-paths-helper"
	"gopkg.in/yaml.v3"
)

type Brick struct {
	Name  string `yaml:"-"` // Ignores this field, to be handled manually
	Model string `yaml:"model,omitempty"`
}

type AppDescriptor struct {
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	Ports       []int   `yaml:"ports"`
	Bricks      []Brick `yaml:"bricks"`
	Icon        string  `yaml:"icon,omitempty"`
}

func (md *Brick) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode: // String type brick (i.e. "- arduino/brickname", without ':').
		md.Name = node.Value

	case yaml.MappingNode: // Map type brick (name followed by a ':' and, optionally, some fields).
		if len(node.Content) != 2 {
			return fmt.Errorf("line %d: expected single-key map for brick item", node.Line)
		}

		keyNode := node.Content[0]
		valueNode := node.Content[1]

		switch {
		case valueNode.Kind == yaml.ScalarNode && valueNode.Value == "":
		case valueNode.Kind == yaml.MappingNode:
			// This alias is used to bypass the custom UnmarshalYAML when decoding the inner details map.
			type brickAlias Brick
			var details brickAlias
			if err := valueNode.Decode(&details); err != nil {
				return fmt.Errorf("line %d: failed to decode brick details map for '%s': %w", valueNode.Line, md.Name, err)
			}
			*md = Brick(details)
		default:
			return fmt.Errorf("line %d: unexpected value type for brick key '%s' (expected map or null, got %v)",
				valueNode.Line, keyNode.Value, valueNode.ShortTag())
		}
		md.Name = keyNode.Value

	default:
		// The node is neither a scalar string nor a map.
		return fmt.Errorf("line %d: expected scalar or mapping node for dependency item, got %v", node.Line, node.ShortTag())
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

	return descriptor, validate(descriptor)
}

func validate(app AppDescriptor) error {
	var allErrors error
	if app.Icon != "" {
		if !isSingleEmoji(app.Icon) {
			allErrors = errors.Join(allErrors, fmt.Errorf("icon %q is not a valid single emoji", app.Icon))
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
