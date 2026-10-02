// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package platform

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPlatformWithOverride(t *testing.T) {
	tmpDir := paths.New(t.TempDir())
	override := Platform{
		FQBN: "some:custom:board",
	}

	f, err := tmpDir.Join("platform.json").Create()
	require.NoError(t, err)
	defer f.Close()
	err = json.NewEncoder(f).Encode(override)
	require.NoError(t, err)

	p := GetPlatform(tmpDir)
	assert.Equal(t, "some:custom:board", p.FQBN)
	assert.Equal(t, "some:custom", p.PlatformID)
}

func TestApplyOverride(t *testing.T) {
	tests := []struct {
		name              string
		content           string
		initial           Platform
		expectedID        string
		expectedBoardName string
		expectErr         bool
	}{
		{
			name:              "Board name and platform id from the FQBN",
			content:           `{"fqbn":"arduino:zephyr:unoq"}`,
			expectedID:        "arduino:zephyr",
			expectedBoardName: "unoq",
		},
		{
			name:              "FQBN with options",
			content:           `{"fqbn":"vendor:arch:board:opt=1"}`,
			expectedID:        "vendor:arch",
			expectedBoardName: "board",
		},
		{
			name:              "Board name specified in the file",
			content:           `{"fqbn":"arduino:zephyr:unoq","board_name":"custom"}`,
			expectedID:        "arduino:zephyr",
			expectedBoardName: "custom",
		},
		{
			name:              "Empty board name specified in the file",
			content:           `{"fqbn":"arduino:zephyr:unoq","board_name":""}`,
			expectedID:        "arduino:zephyr",
			expectedBoardName: "unoq",
		},
		{
			name:              "No FQBN keeps the detected platform",
			content:           `{"board_name":"ventunoq"}`,
			initial:           Platform{FQBN: "arduino:zephyr:unoq", PlatformID: "arduino:zephyr", BoardName: BoardUnoQ},
			expectedID:        "arduino:zephyr",
			expectedBoardName: "ventunoq",
		},
		{
			name:              "No FQBN on an unknown board",
			content:           `{"board_name":"ventunoq"}`,
			expectedID:        "",
			expectedBoardName: "ventunoq",
		},
		{
			name:              "Invalid FQBN does not change the platform id",
			content:           `{"fqbn":"arduino::unoq"}`,
			initial:           Platform{PlatformID: "arduino:zephyr", BoardName: BoardUnoQ},
			expectedID:        "arduino:zephyr",
			expectedBoardName: BoardUnoQ,
		},
		{
			name:      "Invalid JSON",
			content:   `{"fqbn":`,
			initial:   Platform{PlatformID: "arduino:zephyr", BoardName: BoardUnoQ},
			expectErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := paths.New(t.TempDir()).Join("platform.json")
			require.NoError(t, filePath.WriteFile([]byte(tt.content)))

			p := tt.initial
			err := applyOverride(filePath, &p)
			if tt.expectErr {
				require.Error(t, err)
				assert.Equal(t, tt.initial, p)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedID, p.PlatformID)
			assert.Equal(t, tt.expectedBoardName, p.BoardName)
		})
	}
}

func TestParseOSReleaseName(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "ubuntu",
			content:  "PRETTY_NAME=\"Ubuntu 24.04.4 LTS\"\nNAME=\"Ubuntu\"\n",
			expected: "Ubuntu",
		},
		{
			name:     "qualcomm",
			content:  "ID=qcom-distro-sota\nNAME=\"Qualcomm Linux Reference Distro (OTA-enabled)\"\nVERSION=\"2.0\"\n",
			expected: "Qualcomm Linux Reference Distro (OTA-enabled)",
		},
		{
			name:     "unquoted value",
			content:  "NAME=Debian\n",
			expected: "Debian",
		},
		{
			name:     "missing NAME field",
			content:  "ID=debian\nVERSION_ID=12\n",
			expected: "",
		},
		{
			name:     "empty content",
			content:  "",
			expected: "",
		},
		{
			name:     "NAME substring should not match a different key",
			content:  "PRETTY_NAME=\"Some Distro\"\n",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, parseOSReleaseName(strings.NewReader(tc.content)))
		})
	}
}

func TestClassifyDistro(t *testing.T) {
	tests := []struct {
		name          string
		osReleaseName string
		expected      Distro
	}{
		{
			name:          "ubuntu by name",
			osReleaseName: "Ubuntu",
			expected:      DistroUbuntu,
		},
		{
			name:          "ubuntu case insensitive",
			osReleaseName: "UBUNTU",
			expected:      DistroUbuntu,
		},
		{
			name:          "qualcomm by name",
			osReleaseName: "Qualcomm Linux Reference Distro (OTA-enabled)",
			expected:      DistroQLI,
		},
		{
			name:          "qualcomm case insensitive",
			osReleaseName: "qualcomm linux",
			expected:      DistroQLI,
		},
		{
			name:          "debian by name",
			osReleaseName: "Debian GNU/Linux",
			expected:      DistroDebian,
		},
		{
			name:          "ubuntu takes precedence over debian",
			osReleaseName: "Ubuntu",
			expected:      DistroUbuntu,
		},
		{
			name:          "unknown with no matches",
			osReleaseName: "Some Other Distro",
			expected:      DistroUnknown,
		},
		{
			name:          "unknown with empty name",
			osReleaseName: "",
			expected:      DistroUnknown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, classifyDistro(tc.osReleaseName))
		})
	}
}

func TestPackageAndArchitecture(t *testing.T) {
	tests := []struct {
		name         string
		platformID   string
		expectedPkg  string
		expectedArch string
		expectErr    bool
	}{
		{name: "Well formed id", platformID: "arduino:zephyr", expectedPkg: "arduino", expectedArch: "zephyr"},
		{name: "Empty id", platformID: "", expectErr: true},
		{name: "Missing separator", platformID: "arduino", expectErr: true},
		{name: "Empty package", platformID: ":zephyr", expectErr: true},
		{name: "Empty architecture", platformID: "arduino:", expectErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg, arch, err := Platform{PlatformID: tt.platformID}.PackageAndArchitecture()

			if tt.expectErr {
				require.ErrorContains(t, err, "invalid platform id")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedPkg, pkg)
			assert.Equal(t, tt.expectedArch, arch)
		})
	}
}
