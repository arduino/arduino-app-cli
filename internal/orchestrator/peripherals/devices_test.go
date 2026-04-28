// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package peripherals

import (
	"testing"

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

func TestContainsVirtualDevice(t *testing.T) {
	tests := []struct {
		name        string
		deviceClass DeviceClass
		devices     []string
		want        bool
	}{
		{
			name:        "Match found in camera class",
			deviceClass: "camera",
			devices:     []string{"video0", "remote_camera_0", "video1"},
			want:        true,
		},
		{
			name:        "No match in camera class",
			deviceClass: "camera",
			devices:     []string{"video0", "video1"},
			want:        false,
		},
		{
			name:        "Unknown device class",
			deviceClass: "microphone",
			devices:     []string{"remote_mic_0"},
			want:        false,
		},
		{
			name:        "Empty devices list",
			deviceClass: "camera",
			devices:     []string{},
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasVirtualDevice(tt.deviceClass, tt.devices)
			if got != tt.want {
				t.Errorf("HasVirtualDevice() = %v, want %v", got, tt.want)
			}
		})
	}
}
