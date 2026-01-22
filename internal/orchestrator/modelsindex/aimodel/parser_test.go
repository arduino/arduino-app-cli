package aimodel

import (
	"os"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
)

func TestParseModelDescription(t *testing.T) {
	modelDescriptor := `
id: "my-model-id"
name: "my custom model from edge impulse"
description: "A small and accurate model for detecting bounding boxes for faces in images."
bricks:
  - id: "arduino:object-detection"
    model_configuration:
      "EI_OBJ_DETECTION_MODEL": "/models/ootb/ei/lw-face-det.eim"
      "ANOTHER_VARIABLE": true
`
	modelYamlPath := paths.New(t.TempDir(), "model.yaml")
	err := os.WriteFile(modelYamlPath.String(), []byte(modelDescriptor), 0600)
	require.NoError(t, err)

	descr, err := ParseModelDescriptorFile(modelYamlPath)
	require.NoError(t, err)

	require.Equal(t, ModelDescriptor{
		ID:          "my-model-id",
		Name:        "my custom model from edge impulse",
		Description: "A small and accurate model for detecting bounding boxes for faces in images.",
		Bricks: []BrickConfig{
			{
				ID: "arduino:object-detection",
				ModelConfiguration: map[string]any{
					"EI_OBJ_DETECTION_MODEL": "/models/ootb/ei/lw-face-det.eim",
					"ANOTHER_VARIABLE":       true,
				},
			},
		},
	}, descr)

}
