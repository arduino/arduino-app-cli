package bricksindex

import (
	"iter"
	"slices"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/peripherals"
)

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
