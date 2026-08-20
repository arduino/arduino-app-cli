// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

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
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
)

func TestValidateAppDescriptorBricks(t *testing.T) {
	bricksIndex := &bricksindex.BricksIndex{
		BuiltInBricks: []bricksindex.Brick{
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
				ID:           "arduino:ai-brick",
				Name:         "Arduino using an ai model",
				RequireModel: true,
				ModelName:    "i-am-default-model",
			},
			{
				ID:   "arduino:brick-with-hidden-variable",
				Name: "Hidden variable brick",
				Variables: []bricksindex.BrickVariable{
					{
						Name:   "I_AM_HIDDEN_WITHOUT_DEFAULT",
						Hidden: true,
					},
					{
						Name:         "I_AM_HIDDEN_WITH_DEFAULT",
						Hidden:       true,
						DefaultValue: "i-am-the-default-value-of-a-hidden-variable",
					},
				},
			},
		},
	}

	modelIndex := &modelsindex.ModelsIndex{
		InternalModels: []modelsindex.AIModel{
			{
				ID:     "i-am-model-2",
				Status: modelsindex.InstalledStatus,
				Bricks: []modelsindex.BrickConfig{{ID: "arduino:ai-brick"}},
			},
			{
				ID:     "i-am-incompatible-model",
				Status: modelsindex.InstalledStatus,
				Bricks: []modelsindex.BrickConfig{{ID: "arduino:some-other-brick"}},
			},
			{
				ID:     "i-am-not-installed-model",
				Status: modelsindex.NotInstalledStatus,
				Bricks: []modelsindex.BrickConfig{{ID: "arduino:ai-brick"}},
			},
			{
				ID:     "i-am-default-model",
				Status: modelsindex.InstalledStatus,
				Bricks: []modelsindex.BrickConfig{{ID: "arduino:ai-brick"}},
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
		{
			name: "invalid if the model is not compatible with the brick",
			yamlContent: `
name: App with an incompatible model
bricks:
  - arduino:ai-brick:
      model: i-am-incompatible-model
`,
			expectedError: errors.New("model \"i-am-incompatible-model\" is not compatible with brick \"arduino:ai-brick\""),
		},
		{
			name: "invalid if the model is not installed",
			yamlContent: `
name: App with a not installed model
bricks:
  - arduino:ai-brick:
      model: i-am-not-installed-model
`,
			expectedError: errors.New("model \"i-am-not-installed-model\" for brick \"arduino:ai-brick\" is not installed"),
		},
		{
			name: "valid if no model is specified and the brick default model is installed",
			yamlContent: `
name: App using the brick default model
bricks:
  - arduino:ai-brick
`,
			expectedError: nil,
		},
		{
			name: "an hiddden variable with a concrete value does not cause validation error",
			yamlContent: `
name: App with hidden variable with default value
bricks:
  - arduino:brick-with-hidden-variable:
      variables:
        I_AM_HIDDEN_WITHOUT_DEFAULT: "some-value"
`,
			expectedError: nil,
		},
		{
			name: "is required works also for hidden variables",
			yamlContent: `
name: App with hidden variable
bricks:
  - arduino:brick-with-hidden-variable:
`,
			expectedError: errors.New("variable \"I_AM_HIDDEN_WITHOUT_DEFAULT\" is required by brick \"arduino:brick-with-hidden-variable\""),
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

			err = checkBricks(t.Context(), appDescriptor.Bricks, bricksIndex, modelIndex)
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
		BuiltInBricks: []bricksindex.Brick{
			{
				ID:              "arduino:brick-with-camera-device",
				Name:            "a brick that requires a camera",
				RequiredDevices: []peripherals.DeviceClass{peripherals.CameraClass},
			},
			{
				ID:              "arduino:another-brick-with-camera-device",
				Name:            "another brick that requires a camera",
				RequiredDevices: []peripherals.DeviceClass{peripherals.CameraClass},
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

	requiredClasses, err := requiredDeviceClasses(bIndex, appDescriptor.Bricks)
	require.NoError(t, err)

	err = checkRequiredDevices(requiredClasses, availableDevices)
	require.Equal(t, "no camera device found", err.Error())
}

func TestCheckRequiredDevicesNoError(t *testing.T) {
	// do not fail if a brick requires a virtual camera device

	bIndex := &bricksindex.BricksIndex{
		BuiltInBricks: []bricksindex.Brick{
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

	requiredClasses, err := requiredDeviceClasses(bIndex, appDescriptor.Bricks)
	require.NoError(t, err)

	err = checkRequiredDevices(requiredClasses, availableDevices)
	require.NoError(t, err)
}

func TestCheckRequiredDevice(t *testing.T) {
	testCases := []struct {
		name                      string
		brickRequiredDevicesClass []peripherals.DeviceClass
		availableDevices          peripherals.AvailableDevices
		wantErr                   bool
		errMessage                string
	}{
		{
			name:                      "All required devices are available",
			brickRequiredDevicesClass: []peripherals.DeviceClass{peripherals.CameraClass, peripherals.MicrophoneClass, peripherals.SpeakerClass},
			availableDevices: peripherals.AvailableDevices{
				HasSoundDevice: true,
				HasVideoDevice: true,
			},
			wantErr:    false,
			errMessage: "",
		},
		{
			name:                      "Required camera not available",
			brickRequiredDevicesClass: []peripherals.DeviceClass{peripherals.CameraClass},
			availableDevices: peripherals.AvailableDevices{
				HasSoundDevice: true,
				HasVideoDevice: false,
			},
			wantErr:    true,
			errMessage: "no camera device found",
		},
		{
			name:                      "Required microphone not available",
			brickRequiredDevicesClass: []peripherals.DeviceClass{peripherals.MicrophoneClass},
			availableDevices: peripherals.AvailableDevices{
				HasSoundDevice: false,
				HasVideoDevice: true,
			},
			wantErr:    true,
			errMessage: "no microphone device found",
		},
		{
			name:                      "Required speaker not available",
			brickRequiredDevicesClass: []peripherals.DeviceClass{peripherals.SpeakerClass},
			availableDevices: peripherals.AvailableDevices{
				HasSoundDevice: false,
				HasVideoDevice: true,
			},
			wantErr:    true,
			errMessage: "no speaker device found",
		},
		{
			name:                      "Required speaker and camera not available",
			brickRequiredDevicesClass: []peripherals.DeviceClass{peripherals.SpeakerClass, peripherals.CameraClass},
			availableDevices: peripherals.AvailableDevices{
				HasSoundDevice: false,
				HasVideoDevice: false,
			},
			wantErr:    true,
			errMessage: "no camera device found\nno speaker device found",
		},
		{
			name:                      "Required speaker and microphone not available",
			brickRequiredDevicesClass: []peripherals.DeviceClass{peripherals.SpeakerClass, peripherals.MicrophoneClass},
			availableDevices: peripherals.AvailableDevices{
				HasSoundDevice: false,
				HasVideoDevice: false,
			},
			wantErr:    true,
			errMessage: "no microphone device found\nno speaker device found",
		},
		{
			name:                      "Required camera and microphone not available",
			brickRequiredDevicesClass: []peripherals.DeviceClass{peripherals.CameraClass, peripherals.MicrophoneClass},
			availableDevices: peripherals.AvailableDevices{
				HasSoundDevice: false,
				HasVideoDevice: false,
			},
			wantErr:    true,
			errMessage: "no camera device found\nno microphone device found",
		},
		{
			name:                      "No required devices",
			brickRequiredDevicesClass: []peripherals.DeviceClass{},
			availableDevices: peripherals.AvailableDevices{
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
				BuiltInBricks: []bricksindex.Brick{
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

			requiredClasses, err := requiredDeviceClasses(bIndex, appDescriptor.Bricks)
			require.NoError(t, err)

			err = checkRequiredDevices(requiredClasses, tc.availableDevices)
			if tc.wantErr {
				require.Error(t, err, "should have returned an error")
				require.Equal(t, tc.errMessage, err.Error())
			} else {
				require.NoError(t, err, "should not have returned an error")
			}
		})
	}
}

func TestRequiredDeviceClasses(t *testing.T) {
	bIndex := &bricksindex.BricksIndex{
		BuiltInBricks: []bricksindex.Brick{
			{
				ID:              "arduino:camera-brick",
				RequiredDevices: []peripherals.DeviceClass{peripherals.CameraClass},
			},
			{
				ID:              "arduino:audio-brick",
				RequiredDevices: []peripherals.DeviceClass{peripherals.MicrophoneClass, peripherals.SpeakerClass},
			},
			{
				ID: "arduino:plain-brick",
			},
		},
	}

	t.Run("collects the classes required by every brick", func(t *testing.T) {
		bricks := []app.Brick{
			{ID: "arduino:camera-brick"},
			{ID: "arduino:audio-brick"},
		}

		required, err := requiredDeviceClasses(bIndex, bricks)
		require.NoError(t, err)
		require.Equal(t, []peripherals.DeviceClass{
			peripherals.CameraClass,
			peripherals.MicrophoneClass,
			peripherals.SpeakerClass,
		}, required)
	})

	t.Run("skips the classes satisfied by a virtual device", func(t *testing.T) {
		bricks := []app.Brick{
			{ID: "arduino:camera-brick", Devices: []string{"remote_camera_0"}},
		}

		required, err := requiredDeviceClasses(bIndex, bricks)
		require.NoError(t, err)
		require.Empty(t, required)
	})

	t.Run("returns an empty set when no brick requires a device", func(t *testing.T) {
		bricks := []app.Brick{
			{ID: "arduino:plain-brick"},
		}

		required, err := requiredDeviceClasses(bIndex, bricks)
		require.NoError(t, err)
		require.Empty(t, required)
	})

	t.Run("fails on a brick missing from the index", func(t *testing.T) {
		bricks := []app.Brick{
			{ID: "arduino:camera-brick"},
			{ID: "arduino:unknown-brick"},
		}

		required, err := requiredDeviceClasses(bIndex, bricks)
		require.ErrorContains(t, err, "not found")
		require.Nil(t, required)
	})
}

func TestNeedsAudioDevices(t *testing.T) {
	testCases := []struct {
		name     string
		required []peripherals.DeviceClass
		want     bool
	}{
		{name: "microphone", required: []peripherals.DeviceClass{peripherals.MicrophoneClass}, want: true},
		{name: "speaker", required: []peripherals.DeviceClass{peripherals.SpeakerClass}, want: true},
		{name: "camera only", required: []peripherals.DeviceClass{peripherals.CameraClass}, want: false},
		{name: "no device", required: nil, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, needsAudioDevices(tc.required))
		})
	}
}

func TestDetectPortCollisions(t *testing.T) {
	bIndex := &bricksindex.BricksIndex{
		BuiltInBricks: []bricksindex.Brick{
			{ID: "arduino:web_ui", Ports: []string{"7000"}},
			{ID: "arduino:streamlit_ui", Ports: []string{"7000"}},
			{ID: "arduino:data_logger", Ports: []string{"8080", "8080"}},
			{ID: "arduino:object_detection"},
		},
	}

	testCases := []struct {
		name         string
		appPorts     []int
		bricks       []app.Brick
		servicePorts map[string][]string
		want         []portCollision
	}{
		{
			name:   "no collision",
			bricks: []app.Brick{{ID: "arduino:web_ui"}, {ID: "arduino:object_detection"}},
			want:   nil,
		},
		{
			name:   "two bricks on the same port",
			bricks: []app.Brick{{ID: "arduino:web_ui"}, {ID: "arduino:streamlit_ui"}},
			want:   []portCollision{{Port: "7000", Sources: []string{"arduino:web_ui", "arduino:streamlit_ui"}}},
		},
		{
			name:     "app.yaml colliding with a brick",
			appPorts: []int{7000},
			bricks:   []app.Brick{{ID: "arduino:web_ui"}},
			want:     []portCollision{{Port: "7000", Sources: []string{appPortsSource, "arduino:web_ui"}}},
		},
		{
			name:     "duplicated ports within a single source are not a collision",
			appPorts: []int{9000, 9000},
			bricks:   []app.Brick{{ID: "arduino:data_logger"}},
			want:     nil,
		},
		{
			name:   "bricks missing from the index are skipped",
			bricks: []app.Brick{{ID: "arduino:web_ui"}, {ID: "arduino:unknown-brick"}},
			want:   nil,
		},
		{
			name:     "multiple collisions are sorted by port",
			appPorts: []int{7000, 8080},
			bricks:   []app.Brick{{ID: "arduino:web_ui"}, {ID: "arduino:data_logger"}},
			want: []portCollision{
				{Port: "7000", Sources: []string{appPortsSource, "arduino:web_ui"}},
				{Port: "8080", Sources: []string{appPortsSource, "arduino:data_logger"}},
			},
		},
		{
			name:         "a service colliding with a brick",
			bricks:       []app.Brick{{ID: "arduino:web_ui"}},
			servicePorts: map[string][]string{"arduino:proxy": {"7000"}},
			want:         []portCollision{{Port: "7000", Sources: []string{"arduino:web_ui", "arduino:proxy"}}},
		},
		{
			name:         "a service colliding with app.yaml",
			appPorts:     []int{8086},
			servicePorts: map[string][]string{"arduino:tsstore": {"8086"}},
			want:         []portCollision{{Port: "8086", Sources: []string{appPortsSource, "arduino:tsstore"}}},
		},
		{
			name:         "two services on the same port are reported in a stable order",
			servicePorts: map[string][]string{"arduino:b-service": {"9000"}, "arduino:a-service": {"9000"}},
			want:         []portCollision{{Port: "9000", Sources: []string{"arduino:a-service", "arduino:b-service"}}},
		},
		{
			name:         "services not sharing a port are not a collision",
			bricks:       []app.Brick{{ID: "arduino:web_ui"}},
			servicePorts: map[string][]string{"arduino:tsstore": {"8086"}},
			want:         nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, detectPortCollisions(tc.appPorts, tc.bricks, bIndex, tc.servicePorts))
		})
	}
}

func TestCheckPortCollisions(t *testing.T) {
	bIndex := &bricksindex.BricksIndex{
		BuiltInBricks: []bricksindex.Brick{
			{ID: "arduino:web_ui", Ports: []string{"7000"}},
			{ID: "arduino:streamlit_ui", Ports: []string{"7000"}},
			{ID: "arduino:data_logger", Ports: []string{"8080"}},
			{ID: "arduino:object_detection"},
		},
	}

	testCases := []struct {
		name       string
		appPorts   []int
		bricks     []app.Brick
		wantErrors []string
	}{
		{
			name:   "no collision returns no error",
			bricks: []app.Brick{{ID: "arduino:web_ui"}, {ID: "arduino:object_detection"}},
		},
		{
			name:       "two bricks on the same port",
			bricks:     []app.Brick{{ID: "arduino:web_ui"}, {ID: "arduino:streamlit_ui"}},
			wantErrors: []string{`port 7000 is declared by more than one source (arduino:web_ui, arduino:streamlit_ui)`},
		},
		{
			name:     "every collision is reported",
			appPorts: []int{7000, 8080},
			bricks:   []app.Brick{{ID: "arduino:web_ui"}, {ID: "arduino:data_logger"}},
			wantErrors: []string{
				`port 7000 is declared by more than one source (app.yaml, arduino:web_ui)`,
				`port 8080 is declared by more than one source (app.yaml, arduino:data_logger)`,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkPortCollisions(tc.appPorts, tc.bricks, bIndex, &servicesindex.ServicesIndex{})
			if len(tc.wantErrors) == 0 {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, want := range tc.wantErrors {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}
