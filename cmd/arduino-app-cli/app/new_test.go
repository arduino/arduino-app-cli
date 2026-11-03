// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func TestNewCreateCmdBricks(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "no bricks",
			want: []string{},
		},
		{
			name: "single brick",
			args: []string{"--bricks", "arduino:web_ui"},
			want: []string{"arduino:web_ui"},
		},
		{
			name: "comma-separated bricks",
			args: []string{"--bricks", "arduino:web_ui,arduino:tts"},
			want: []string{"arduino:web_ui", "arduino:tts"},
		},
		{
			name: "shorthand comma-separated bricks",
			args: []string{"-b", "arduino:web_ui,arduino:tts"},
			want: []string{"arduino:web_ui", "arduino:tts"},
		},
		{
			name: "equals comma-separated bricks",
			args: []string{"--bricks=arduino:web_ui,arduino:tts"},
			want: []string{"arduino:web_ui", "arduino:tts"},
		},
		{
			name: "repeated flags",
			args: []string{"--bricks", "arduino:web_ui", "--bricks", "arduino:tts"},
			want: []string{"arduino:web_ui", "arduino:tts"},
		},
		{
			name: "mixed comma-separated and repeated flags",
			args: []string{"--bricks", "arduino:web_ui,arduino:tts", "-b", "arduino:arduino_cloud"},
			want: []string{"arduino:web_ui", "arduino:tts", "arduino:arduino_cloud"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newCreateCmd(config.Configuration{})
			require.NoError(t, cmd.ParseFlags(tt.args))

			brickIDs, err := cmd.Flags().GetStringSlice("bricks")
			require.NoError(t, err)
			require.Equal(t, tt.want, brickIDs)
		})
	}
}
