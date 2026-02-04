package app

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
)

func TestValidateAppDescriptorBricks(t *testing.T) {
	bricksIndex := &bricksindex.BricksIndex{
		Bricks: []bricksindex.Brick{
			{
				ID:          "arduino:arduino_cloud",
				Name:        "Arduino Cloud",
				Description: "Connects to Arduino Cloud",
				Variables: []bricksindex.BrickVariable{
					{
						Name:         "ARDUINO_DEVICE_ID",
						Description:  "Arduino Cloud Device ID",
						DefaultValue: "", // Required (no default value)
					},
					{
						Name:         "ARDUINO_SECRET",
						Description:  "Arduino Cloud Secret",
						DefaultValue: "", // Required (no default value)
					},
				},
			},
			{
				ID:        "arduino:ai-brick",
				Name:      "Arduino using an ai model",
				ModelName: "i-am-default-model",
			},
		},
	}

	modelIndex := &modelsindex.ModelsIndex{
		InternalModels: []modelsindex.AIModel{
			{
				ID: "i-am-model-2",
			},
		},
	}

	testCases := []struct {
		name          string
		yamlContent   string
		expectedError error
	}{
		{
			name: "valid with all required filled",
			yamlContent: `
name: App ok
description: App ok
bricks:
  - arduino:arduino_cloud:
      variables:
        ARDUINO_DEVICE_ID: "my-device-id"
        ARDUINO_SECRET: "my-secret"
`,
			expectedError: nil,
		},
		{
			name: "valid with missing bricks",
			yamlContent: `
name: App with no bricks
description: App with no bricks description
`,
			expectedError: nil,
		},
		{
			name: "valid with empty list of bricks",
			yamlContent: `
name: App with empty bricks
description: App with empty bricks

bricks: []
`,
			expectedError: nil,
		},
		{
			name: "valid if required variable is empty string",
			yamlContent: `
name: App with an empty variable
description: App with an empty variable
bricks:
  - arduino:arduino_cloud:
      variables:
        ARDUINO_DEVICE_ID: "my-device-id"
        ARDUINO_SECRET:
`,
			expectedError: nil,
		},
		{
			name: "invalid if required variable is omitted",
			yamlContent: `
name: App with no required variables
description: App with no required variables
bricks:
  - arduino:arduino_cloud
`,
			expectedError: errors.Join(
				errors.New("variable \"ARDUINO_DEVICE_ID\" is required by brick \"arduino:arduino_cloud\""),
				errors.New("variable \"ARDUINO_SECRET\" is required by brick \"arduino:arduino_cloud\""),
			),
		},
		{
			name: "invalid if a required variable among two is omitted",
			yamlContent: `
name: App only one required variable filled
description: App only one required variable filled
bricks:
  - arduino:arduino_cloud:
      variables:
        ARDUINO_DEVICE_ID: "my-device-id"
`,
			expectedError: errors.New("variable \"ARDUINO_SECRET\" is required by brick \"arduino:arduino_cloud\""),
		},
		{
			name: "invalid if brick id not found",
			yamlContent: `
name: App no existing brick
description: App no existing brick
bricks:
  - arduino:not_existing_brick:
      variables:
        ARDUINO_DEVICE_ID: "my-device-id"
        ARDUINO_SECRET: "LAKDJ"
`,
			expectedError: errors.New("brick \"arduino:not_existing_brick\" not found"),
		},
		{
			name: "log a warning if variable does not exist in the brick",
			yamlContent: `
name: App with non existing variable
description: App with non existing variable
bricks:
  - arduino:arduino_cloud:
      variables:
        NOT_EXISTING_VARIABLE: "this-is-a-not-existing-variable-for-the-brick"
        ARDUINO_DEVICE_ID: "my-device-id"
        ARDUINO_SECRET: "my-secret"
`,
			expectedError: nil,
		},
		{
			name: "invalid if the model id does not exist",
			yamlContent: `
name: App with using a not found model
bricks:
  - arduino:ai-brick:
      model: a-not-existing-model
`,
			expectedError: errors.New("model \"a-not-existing-model\" for brick \"arduino:ai-brick\" not found"),
		},
		{
			name: "valid if the model exist",
			yamlContent: `
name: App with a valid model
bricks:
  - arduino:ai-brick:
      model: i-am-model-2
`,
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			err := paths.New(tempDir).MkdirAll()
			require.NoError(t, err)
			appYaml := paths.New(tempDir, "app.yaml")
			err = os.WriteFile(appYaml.String(), []byte(tc.yamlContent), 0600)
			require.NoError(t, err)

			appDescriptor, err := ParseDescriptorFile(appYaml)
			require.NoError(t, err)

			err = ValidateBricks(appDescriptor, bricksIndex, modelIndex)
			if tc.expectedError == nil {
				assert.NoError(t, err, "Expected no validation errors")
			} else {
				require.Error(t, err, "Expected validation error")
				assert.Equal(t, tc.expectedError.Error(), err.Error(), "Error message should match")
			}
		})
	}
}
