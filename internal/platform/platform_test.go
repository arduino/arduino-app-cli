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

func TestEIDeviceType(t *testing.T) {
	tests := []struct {
		boardName string
		want      string
	}{
		{"unoq", "runner-linux-aarch64"},
		{"ventunoq", "runner-linux-aarch64-qnn"},
		{"", "runner-linux-aarch64"},
	}
	for _, tt := range tests {
		t.Run(tt.boardName, func(t *testing.T) {
			p := Platform{BoardName: tt.boardName}
			assert.Equal(t, tt.want, p.EIDeviceType())
		})
	}
}

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
