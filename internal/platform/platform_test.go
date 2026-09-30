// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package platform

import (
	"encoding/json"
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
			expectedBoardName: "",
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
