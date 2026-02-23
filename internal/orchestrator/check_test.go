package orchestrator

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/peripherals"
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

			appDescriptor, err := app.ParseDescriptorFile(appYaml)
			require.NoError(t, err)

			err = checkBricks(appDescriptor, bricksIndex, modelIndex)
			if tc.expectedError == nil {
				assert.NoError(t, err, "Expected no validation errors")
			} else {
				require.Error(t, err, "Expected validation error")
				assert.Equal(t, tc.expectedError.Error(), err.Error(), "Error message should match")
			}
		})
	}
}

func TestValidateVirtualDevice(t *testing.T) {
	// fail if a camera device is not detected and one of two brick require a physical camera

	bIndex := &bricksindex.BricksIndex{
		Bricks: []bricksindex.Brick{
			{
				ID:              "arduino:brick-with-camera-device",
				Name:            "a brick that requires a camera",
				RequiredDevices: []string{"camera"},
			},
			{
				ID:              "arduino:another-brick-with-camera-device",
				Name:            "another brick that requires a camera",
				RequiredDevices: []string{"camera"},
			},
		},
	}

	appDescriptor := app.AppDescriptor{
		Bricks: []app.Brick{
			{
				ID:      "arduino:brick-with-camera-device",
				Devices: []string{"remote_camera_0"},
			},
			{
				ID: "arduino:another-brick-with-camera-device",
			},
		},
	}

	availableDevices := peripherals.AvailableDevices{
		HasVideoDevice: false,
	}

	err := checkRequiredDevices(bIndex, appDescriptor.Bricks, availableDevices)
	require.Equal(t, "no camera device found", err.Error())
}

func TestCheckRequiredDevicesNoError(t *testing.T) {
	// do not fail if a brick requires a virtual camera device

	bIndex := &bricksindex.BricksIndex{
		Bricks: []bricksindex.Brick{
			{
				ID:   "arduino:brick-with-camera-device",
				Name: "a brick that requires a camera",
			},
		},
	}

	appDescriptor := app.AppDescriptor{
		Bricks: []app.Brick{
			{
				ID:      "arduino:brick-with-camera-device",
				Devices: []string{"remote_camera_0"},
			},
		},
	}

	availableDevices := peripherals.AvailableDevices{
		HasVideoDevice: false,
	}

	err := checkRequiredDevices(bIndex, appDescriptor.Bricks, availableDevices)
	require.NoError(t, err)
}

func TestCheckRequiredDevice(t *testing.T) {
	testCases := []struct {
		name                      string
		brickRequiredDevicesClass []string
		availableDevices          peripherals.AvailableDevices
		wantErr                   bool
		errMessage                string
	}{
		{
			name:                      "All required devices are available",
			brickRequiredDevicesClass: []string{"camera", "microphone", "speaker"},
			availableDevices: peripherals.AvailableDevices{
				HasSoundDevice: true,
				HasVideoDevice: true,
			},
			wantErr:    false,
			errMessage: "",
		},
		{
			name:                      "Required camera not available",
			brickRequiredDevicesClass: []string{"camera"},
			availableDevices: peripherals.AvailableDevices{
				HasSoundDevice: true,
				HasVideoDevice: false,
			},
			wantErr:    true,
			errMessage: "no camera device found",
		},
		{
			name:                      "Required microphone not available",
			brickRequiredDevicesClass: []string{"microphone"},
			availableDevices: peripherals.AvailableDevices{
				HasSoundDevice: false,
				HasVideoDevice: true,
			},
			wantErr:    true,
			errMessage: "no microphone device found",
		},
		{
			name:                      "Required speaker not available",
			brickRequiredDevicesClass: []string{"speaker"},
			availableDevices: peripherals.AvailableDevices{
				HasSoundDevice: false,
				HasVideoDevice: true,
			},
			wantErr:    true,
			errMessage: "no speaker device found",
		},
		{
			name:                      "Required speaker and camera not available",
			brickRequiredDevicesClass: []string{"speaker", "camera"},
			availableDevices: peripherals.AvailableDevices{
				HasSoundDevice: false,
				HasVideoDevice: false,
			},
			wantErr:    true,
			errMessage: "no camera device found\nno speaker device found",
		},
		{
			name:                      "Required speaker and microphone not available",
			brickRequiredDevicesClass: []string{"speaker", "microphone"},
			availableDevices: peripherals.AvailableDevices{
				HasSoundDevice: false,
				HasVideoDevice: false,
			},
			wantErr:    true,
			errMessage: "no microphone device found\nno speaker device found",
		},
		{
			name:                      "Required camera and microphone not available",
			brickRequiredDevicesClass: []string{"camera", "microphone"},
			availableDevices: peripherals.AvailableDevices{
				HasSoundDevice: false,
				HasVideoDevice: false,
			},
			wantErr:    true,
			errMessage: "no camera device found\nno microphone device found",
		},
		{
			name:                      "No required devices",
			brickRequiredDevicesClass: []string{},
			availableDevices: peripherals.AvailableDevices{
				DevicePaths:    []string{},
				HasSoundDevice: false,
				HasVideoDevice: true,
			},
			wantErr:    false,
			errMessage: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			bIndex := &bricksindex.BricksIndex{
				Bricks: []bricksindex.Brick{
					{
						ID:              "arduino:a-simple-brick",
						Name:            "a brick to test devices",
						RequiredDevices: tc.brickRequiredDevicesClass,
					},
				},
			}

			appDescriptor := app.AppDescriptor{
				Bricks: []app.Brick{
					{
						ID: "arduino:a-simple-brick"},
				},
			}

			err := checkRequiredDevices(bIndex, appDescriptor.Bricks, tc.availableDevices)
			if tc.wantErr {
				require.Error(t, err, "should have returned an error")
				require.Equal(t, tc.errMessage, err.Error())
			} else {
				require.NoError(t, err, "should not have returned an error")
			}
		})
	}
}
