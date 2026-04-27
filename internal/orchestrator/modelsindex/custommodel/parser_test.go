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
