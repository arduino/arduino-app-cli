package peripherals

import (
	"testing"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSortV4LVideoDevices(t *testing.T) {

	devices := []string{
		"usb-Generic_GENERAL_-_UVC-video-index1",
		"usb-Generic_GENERAL_-_UVC-video-index0",
		"usb-046d_0825-video-index2",
	}

	sortV4lByIndexDevices(devices)
	assert.Equal(t, "usb-Generic_GENERAL_-_UVC-video-index0", devices[0])
	assert.Equal(t, "usb-Generic_GENERAL_-_UVC-video-index1", devices[1])
	assert.Equal(t, "usb-046d_0825-video-index2", devices[2])
}

func TestValidateRequiredDevice(t *testing.T) {
	testCases := []struct {
		name            string
		requiredDevices []string
		devicePaths     []string
		hasVideoDevice  bool
		hasSoundDevice  bool
		hasGPUDevice    bool
		errMessage      string
	}{
		{
			name:            "All required devices are available",
			requiredDevices: []string{"camera", "microphone", "speaker"},
			devicePaths:     []string{"/dev/video0", "/dev/video1", "/dev/snd/pcmC0D0p"},
			hasVideoDevice:  true,
			hasSoundDevice:  true,
			hasGPUDevice:    true,
			errMessage:      "",
		},
		{
			name:            "Required camera not available",
			requiredDevices: []string{"camera"},
			devicePaths:     []string{"/dev/snd/pcmC0D0p"},
			hasVideoDevice:  false,
			hasSoundDevice:  true,
			hasGPUDevice:    true,
			errMessage:      "no camera device found",
		},
		{
			name:            "Required microphone not available",
			requiredDevices: []string{"microphone"},
			devicePaths:     []string{"/dev/snd/pcmC0D0p"},
			hasVideoDevice:  true,
			hasSoundDevice:  false,
			hasGPUDevice:    true,
			errMessage:      "no microphone device found\nno speaker device found",
		},
		{
			name:            "Required speaker not available",
			requiredDevices: []string{"speaker"},
			devicePaths:     []string{"/dev/video0"},
			hasVideoDevice:  true,
			hasSoundDevice:  false,
			hasGPUDevice:    true,
			errMessage:      "no microphone device found\nno speaker device found",
		},
		{
			name:            "No required devices",
			requiredDevices: []string{},
			devicePaths:     []string{},
			hasVideoDevice:  false,
			hasSoundDevice:  false,
			hasGPUDevice:    false,
			errMessage:      "no camera device found\nno microphone device found\nno speaker device found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			appDescriptor := app.AppDescriptor{
				Bricks: []app.Brick{
					{
						Devices: []string{"camera", "microphone", "speaker"},
					},
				},
			}
			availableDevices := AvailableDevices{
				DevicePaths:    tc.devicePaths,
				HasGPUDevice:   tc.hasGPUDevice,
				HasSoundDevice: tc.hasSoundDevice,
				HasVideoDevice: tc.hasVideoDevice,
			}

			err := ValidateRequiredDevices(appDescriptor, &availableDevices)
			if err != nil {
				require.Equal(t, tc.errMessage, err.Error())
			}
		})
	}
}

func TestExtractIndexFromVideoDeviceName(t *testing.T) {
	testCases := []struct {
		name       string
		device     string
		expected   int
		errMessage string
	}{
		{
			name:       "Valid index",
			device:     "usb-Generic_GENERAL_-_UVC-video-index0",
			expected:   0,
			errMessage: "",
		},
		{
			name:       "Invalid index",
			device:     "usb-Generic_GENERAL_-_UVC-video-index",
			expected:   -1,
			errMessage: "strconv.Atoi: parsing \"\": invalid syntax",
		},
		{
			name:       "Missing index",
			device:     "usb",
			expected:   -1,
			errMessage: "substring 'index' not found in \"usb\"",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := extractIndexFromVideoDeviceName(tc.device)
			if tc.errMessage != "" {
				require.Equal(t, tc.errMessage, err.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, actual)
			}
		})
	}

}
