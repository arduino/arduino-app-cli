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
