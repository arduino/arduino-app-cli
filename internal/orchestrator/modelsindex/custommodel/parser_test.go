// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package custommodel

import (
	"os"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
)

func TestParseModelDescription(t *testing.T) {
	modelDescriptor := `
id: "my-model-id"
name: "my custom model name"
runner: "bricks"
description: "A small and accurate description."
bricks:
  - id: "arduino:a-brick-id"
    model_configuration:
      "A_STRING_VARIABLE": "i-am-a-string"
      "A_BOOL_VARIABLE": true
  - id: "arduino:another-brick-id"
    model_configuration:
      "A_STRING_VARIABLE": "i-am-a-string"
      "A_BOOL_VARIABLE": false
metadata:
  a-string-metadata: "a-string-value"
  a-int-metadata: 717280
`
	modelYamlPath := paths.New(t.TempDir(), "model.yaml")
	err := os.WriteFile(modelYamlPath.String(), []byte(modelDescriptor), 0600)
	require.NoError(t, err)

	descr, err := ParseModelDescriptorFile(modelYamlPath)
	require.NoError(t, err)

	require.Equal(t, ModelDescriptor{
		ID:          "my-model-id",
		Name:        "my custom model name",
		Runner:      "bricks",
		Description: "A small and accurate description.",
		Bricks: []BrickConfig{
			{
				ID: "arduino:a-brick-id",
				ModelConfiguration: map[string]string{
					"A_STRING_VARIABLE": "i-am-a-string",
					"A_BOOL_VARIABLE":   "true",
				},
			},
			{
				ID: "arduino:another-brick-id",
				ModelConfiguration: map[string]string{
					"A_STRING_VARIABLE": "i-am-a-string",
					"A_BOOL_VARIABLE":   "false",
				},
			},
		},
		Metadata: map[string]string{
			"a-string-metadata": "a-string-value",
			"a-int-metadata":    "717280",
		},
	}, descr)

}

func TestModelDescriptorValidate(t *testing.T) {
	validEdgeImpulseMetadata := map[string]string{
		"source":                "edgeimpulse",
		"ei-project-id":         "123",
		"ei-impulse-id":         "456",
		"ei-impulse-name":       "my-impulse",
		"ei-deployment-version": "1",
		"ei-model-type":         "float32",
		"ei-engine":             "tflite",
	}

	tests := []struct {
		name        string
		descriptor  ModelDescriptor
		expectError bool
		errorMsgs   []string
	}{
		{
			name: "valid descriptor without source",
			descriptor: ModelDescriptor{
				ID:   "my-id",
				Name: "my-name",
			},
			expectError: false,
		},
		{
			name: "valid descriptor with edgeimpulse source",
			descriptor: ModelDescriptor{
				ID:       "my-id",
				Name:     "my-name",
				Metadata: validEdgeImpulseMetadata,
			},
			expectError: false,
		},
		{
			name: "missing id",
			descriptor: ModelDescriptor{
				Name: "my-name",
			},
			expectError: true,
			errorMsgs:   []string{"invalid model descriptor: id is empty"},
		},
		{
			name: "missing name",
			descriptor: ModelDescriptor{
				ID: "my-id",
			},
			expectError: true,
			errorMsgs:   []string{"invalid model descriptor: name is empty"},
		},
		{
			name:        "missing id and name",
			descriptor:  ModelDescriptor{},
			expectError: true,
			errorMsgs: []string{
				"invalid model descriptor: id is empty",
				"invalid model descriptor: name is empty",
			},
		},
		{
			name: "unsupported source",
			descriptor: ModelDescriptor{
				ID:   "my-id",
				Name: "my-name",
				Metadata: map[string]string{
					"source": "unknown-source",
				},
			},
			expectError: true,
			errorMsgs:   []string{"invalid model descriptor: unsupported source 'unknown-source'"},
		},
		{
			name: "edgeimpulse missing required fields",
			descriptor: ModelDescriptor{
				ID:   "my-id",
				Name: "my-name",
				Metadata: map[string]string{
					"source":        "edgeimpulse",
					"ei-model-type": "float32",
					"ei-engine":     "tflite",
				},
			},
			expectError: true,
			errorMsgs: []string{
				"invalid Edge Impulse metadata: missing required field 'ei-project-id'",
				"invalid Edge Impulse metadata: missing required field 'ei-impulse-id'",
				"invalid Edge Impulse metadata: missing required field 'ei-impulse-name'",
				"invalid Edge Impulse metadata: missing required field 'ei-deployment-version'",
			},
		},
		{
			name: "edgeimpulse unsupported model type",
			descriptor: ModelDescriptor{
				ID:   "my-id",
				Name: "my-name",
				Metadata: map[string]string{
					"source":                "edgeimpulse",
					"ei-project-id":         "123",
					"ei-impulse-id":         "456",
					"ei-impulse-name":       "my-impulse",
					"ei-deployment-version": "1",
					"ei-model-type":         "int8",
					"ei-engine":             "tflite",
				},
			},
			expectError: true,
			errorMsgs:   []string{"invalid Edge Impulse metadata: unsupported model type"},
		},
		{
			name: "edgeimpulse unsupported engine",
			descriptor: ModelDescriptor{
				ID:   "my-id",
				Name: "my-name",
				Metadata: map[string]string{
					"source":                "edgeimpulse",
					"ei-project-id":         "123",
					"ei-impulse-id":         "456",
					"ei-impulse-name":       "my-impulse",
					"ei-deployment-version": "1",
					"ei-model-type":         "float32",
					"ei-engine":             "onnx",
				},
			},
			expectError: true,
			errorMsgs:   []string{"invalid Edge Impulse metadata: unsupported engine"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.descriptor.Validate()
			if tt.expectError {
				require.Error(t, err)
				for _, msg := range tt.errorMsgs {
					require.ErrorContains(t, err, msg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
