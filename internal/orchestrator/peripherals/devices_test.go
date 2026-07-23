// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package peripherals

import (
	"os"
	"path/filepath"
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

func TestDetectCSICameraDriver(t *testing.T) {
	tests := []struct {
		name       string
		drivers    []string
		camxSocket bool
		want       CSICameraDriver
	}{
		{
			name:    "camss driver present",
			drivers: []string{"qcom-camss"},
			want:    CSICameraDriverCamss,
		},
		{
			name:       "camx drivers with socket",
			drivers:    []string{"cam_sync", "cam_smmu", "other-driver"},
			camxSocket: true,
			want:       CSICameraDriverCamx,
		},
		{
			name:       "camx drivers without socket",
			drivers:    []string{"cam_sync", "cam_smmu"},
			camxSocket: false,
			want:       CSICameraDriverNone,
		},
		{
			name:       "socket without camx drivers",
			drivers:    []string{"other-driver"},
			camxSocket: true,
			want:       CSICameraDriverNone,
		},
		{
			name: "no drivers",
			want: CSICameraDriverNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driversDir := t.TempDir()
			for _, d := range tt.drivers {
				require.NoError(t, os.Mkdir(filepath.Join(driversDir, d), 0o755))
			}
			socketPath := filepath.Join(t.TempDir(), "le_cam_socket")
			if tt.camxSocket {
				require.NoError(t, os.WriteFile(socketPath, nil, 0o600))
			}
			assert.Equal(t, tt.want, detectCSICameraDriver(driversDir, socketPath))
		})
	}
}

func TestDetectCSICameraDriverMissingDriversDir(t *testing.T) {
	assert.Equal(t, CSICameraDriverNone, detectCSICameraDriver(filepath.Join(t.TempDir(), "missing"), filepath.Join(t.TempDir(), "le_cam_socket")))
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
