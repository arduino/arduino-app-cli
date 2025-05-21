package parser

import (
	"fmt"

	"github.com/arduino/go-paths-helper"
	"gopkg.in/yaml.v3"
)

type Brick struct {
	Name  string `yaml:"-"` // Ignores this field, to be handled manually
	Model string `yaml:"model,omitempty"`
}

type AppDescriptor struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Ports       []int    `yaml:"ports"`
	Bricks      []Brick  `yaml:"bricks"`
	Categories  []string `yaml:"categories"`
	Icon        string   `yaml:"icon,omitempty"`
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
	data, err := file.ReadFile()
	if err != nil {
		return AppDescriptor{}, err
	}
	descriptor := AppDescriptor{}
	if err := yaml.Unmarshal(data, &descriptor); err != nil {
		return AppDescriptor{}, err
	}
	return descriptor, nil
}
