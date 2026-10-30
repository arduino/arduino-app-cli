// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"testing"

	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/stretchr/testify/assert"
)

func TestIsPlatformPinned(t *testing.T) {
	tests := []struct {
		name      string
		platforms []*rpc.ProfilePlatformReference
		want      bool
	}{
		{
			name:      "platform without version",
			platforms: []*rpc.ProfilePlatformReference{{Id: "arduino:zephyr"}},
			want:      false,
		},
		{
			name:      "platform with version",
			platforms: []*rpc.ProfilePlatformReference{{Id: "arduino:zephyr", Version: new("0.56.0")}},
			want:      true,
		},
		{
			name:      "another platform with version",
			platforms: []*rpc.ProfilePlatformReference{{Id: "arduino:avr", Version: new("1.8.6")}, {Id: "arduino:zephyr"}},
			want:      false,
		},
		{
			name:      "no platforms",
			platforms: nil,
			want:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isPlatformPinned(tt.platforms, "arduino:zephyr"))
		})
	}
}

func TestInstalledPlatformLibraries(t *testing.T) {
	zephyr := &rpc.PlatformSummary{
		Metadata:         &rpc.PlatformMetadata{Id: "arduino:zephyr"},
		InstalledVersion: "1.0.0",
		Releases: map[string]*rpc.PlatformRelease{
			"0.53.0": {Version: "0.53.0"},
			"1.0.0": {
				Version: "1.0.0",
				LibraryDependencies: []*rpc.PlatformLibraryDependency{
					{Name: "Arduino_RouterBridge", Version: "0.4.4"},
					{Name: "ArduinoJson"},
				},
			},
		},
	}

	tests := []struct {
		name      string
		summaries []*rpc.PlatformSummary
		want      []string
	}{
		{
			name:      "libraries of the installed release",
			summaries: []*rpc.PlatformSummary{{Metadata: &rpc.PlatformMetadata{Id: "arduino:avr"}}, zephyr},
			want:      []string{"Arduino_RouterBridge", "ArduinoJson"},
		},
		{
			name: "installed release without libraries",
			summaries: []*rpc.PlatformSummary{{
				Metadata:         zephyr.Metadata,
				InstalledVersion: "0.53.0",
				Releases:         zephyr.Releases,
			}},
			want: nil,
		},
		{
			name: "platform not installed",
			summaries: []*rpc.PlatformSummary{{
				Metadata: zephyr.Metadata,
				Releases: zephyr.Releases,
			}},
			want: nil,
		},
		{
			name:      "platform not found",
			summaries: []*rpc.PlatformSummary{{Metadata: &rpc.PlatformMetadata{Id: "arduino:avr"}}},
			want:      nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, installedPlatformLibraries(tt.summaries, "arduino:zephyr"))
		})
	}
}
