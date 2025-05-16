package parser

import (
	"fmt"

	"github.com/arduino/go-paths-helper"
	"gopkg.in/yaml.v3"
)

type ModuleDependency struct {
	Name  string `yaml:"-"` // Ignores this field, to be handled manually
	Model string `yaml:"model,omitempty"`
}

type Descriptor struct {
	DisplayName        string             `yaml:"display-name"`
	Description        string             `yaml:"description"`
	Ports              []int              `yaml:"ports"`
	ModuleDependencies []ModuleDependency `yaml:"module-dependencies"`
	Categories         []string           `yaml:"categories"`
	Icon               string             `yaml:"icon,omitempty"`
}

func (md *ModuleDependency) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode: // String type dependency (i.e. "- arduino/dependency", without ':').
		md.Name = node.Value

	case yaml.MappingNode: // Map type dependency (name followed by a ':' and, optionally, some fields).
		if len(node.Content) != 2 {
			return fmt.Errorf("line %d: expected single-key map for dependency item", node.Line)
		}

		keyNode := node.Content[0]
		valueNode := node.Content[1]

		switch {
		case valueNode.Kind == yaml.ScalarNode && valueNode.Value == "":
		case valueNode.Kind == yaml.MappingNode:
			// This alias is used to bypass the custom UnmarshalYAML when decoding the inner details map.
			type moduleDependencyAlias ModuleDependency
			var details moduleDependencyAlias
			if err := valueNode.Decode(&details); err != nil {
				return fmt.Errorf("line %d: failed to decode dependency details map for '%s': %w", valueNode.Line, md.Name, err)
			}
			*md = ModuleDependency(details)
		default:
			return fmt.Errorf("line %d: unexpected value type for dependency key '%s' (expected map or null, got %v)",
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
func ParseDescriptorFile(file *paths.Path) (Descriptor, error) {
	data, err := file.ReadFile()
	if err != nil {
		return Descriptor{}, err
	}
	descriptor := Descriptor{}
	if err := yaml.Unmarshal(data, &descriptor); err != nil {
		return Descriptor{}, err
	}
	return descriptor, nil
}
