// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package appid

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

var unoQPlatform = platform.Platform{BoardName: "unoq"}

func TestNewIDFromPath(t *testing.T) {
	tmp := paths.New(t.TempDir())
	t.Setenv("ARDUINO_APP_CLI__APPS_DIR", tmp.Join("apps").String())
	t.Setenv("ARDUINO_APP_CLI__DATA_DIR", tmp.Join("data").String())

	orchestratorConfig, err := config.NewFromEnv()
	require.NoError(t, err)
	examplesUnoQDir := orchestratorConfig.DataDir().Join("examples", "inspirational", "platform_unoq")
	examplesCommonDir := orchestratorConfig.DataDir().Join("examples", "inspirational", "common")
	examplesBaseDir := orchestratorConfig.DataDir().Join("examples")
	require.NoError(t, orchestratorConfig.AppsDir().Join("user-app").MkdirAll())
	require.NoError(t, examplesCommonDir.Join("example-app").MkdirAll())
	require.NoError(t, examplesUnoQDir.Join("special-example-app").MkdirAll())
	require.NoError(t, examplesCommonDir.Join("duplicated-example-app").MkdirAll())
	require.NoError(t, examplesUnoQDir.Join("duplicated-example-app").MkdirAll())
	require.NoError(t, examplesCommonDir.Join("nested", "deep", "example-app").MkdirAll())
	require.NoError(t, examplesUnoQDir.Join("nested", "platform-example").MkdirAll())
	require.NoError(t, examplesBaseDir.Join("core-and-foundational", "nested1", "nested2").MkdirAll())
	require.NoError(t, examplesBaseDir.Join("bricks", "nested1").MkdirAll())
	require.NoError(t, tmp.Join("other-location-app").MkdirAll())

	idProvider := NewAppProvider(orchestratorConfig, unoQPlatform)

	tests := []struct {
		name    string
		in      *paths.Path
		want    ID
		wantErr bool
	}{
		{
			name: "valid user id",
			in:   orchestratorConfig.AppsDir().Join("user-app"),
			want: f.Must(idProvider.ParseID("user:user-app")),
		},
		{
			name: "valid example id",
			in:   examplesCommonDir.Join("example-app"),
			want: f.Must(idProvider.ParseID("examples:example-app")),
		},
		{
			name: "duplicated example id, the platform specific wins",
			in:   examplesUnoQDir.Join("duplicated-example-app"),
			want: f.Must(idProvider.ParseID("examples:duplicated-example-app")),
		},
		{
			name: "platform specific valid example id",
			in:   examplesUnoQDir.Join("special-example-app"),
			want: f.Must(idProvider.ParseID("examples:special-example-app")),
		},
		{
			name: "nested common example id",
			in:   examplesCommonDir.Join("nested", "deep", "example-app"),
			want: f.Must(idProvider.ParseID("examples:nested/deep/example-app")),
		},
		{
			name: "nested platform example id",
			in:   examplesUnoQDir.Join("nested", "platform-example"),
			want: f.Must(idProvider.ParseID("examples:nested/platform-example")),
		},
		{
			name: "valid absolute path",
			in:   tmp.Join("other-location-app"),
			want: f.Must(idProvider.ParseID(tmp.Join("other-location-app").String())),
		},
		{
			name: "inspirational path",
			in:   examplesBaseDir.Join("core-and-foundational"),
			want: f.Must(idProvider.ParseID(examplesBaseDir.Join("core-and-foundational").String())),
		},
		{
			name: "inspirational path",
			in:   examplesBaseDir.Join("core-and-foundational", "nested1", "nested2"),
			want: f.Must(idProvider.ParseID(examplesBaseDir.Join("core-and-foundational", "nested1", "nested2").String())),
		},
		{
			name: "bricks",
			in:   examplesBaseDir.Join("bricks", "nested1"),
			want: f.Must(idProvider.ParseID(examplesBaseDir.Join("bricks", "nested1").String())),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := idProvider.IDFromPath(tt.in)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseID(t *testing.T) {
	tmp := paths.New(t.TempDir())
	t.Setenv("ARDUINO_APP_CLI__APPS_DIR", tmp.Join("apps").String())
	t.Setenv("ARDUINO_APP_CLI__DATA_DIR", tmp.Join("data").String())

	orchestratorConfig, err := config.NewFromEnv()
	require.NoError(t, err)
	require.NoError(t, tmp.Join("other-app").MkdirAll())
	examplesUnoQDir := orchestratorConfig.DataDir().Join("examples", "inspirational", "platform_unoq")
	examplesCommonDir := orchestratorConfig.DataDir().Join("examples", "inspirational", "common")
	examplesBaseDir := orchestratorConfig.DataDir().Join("examples")
	require.NoError(t, examplesCommonDir.Join("example-app").MkdirAll())
	require.NoError(t, examplesCommonDir.Join("nested", "deep", "example-app").MkdirAll())
	require.NoError(t, examplesUnoQDir.Join("nested", "platform-example").MkdirAll())

	idProvider := NewAppProvider(orchestratorConfig, unoQPlatform)

	tests := []struct {
		name     string
		in       string
		wantPath *paths.Path
		wantErr  bool
	}{
		{
			name:     "valid user id",
			in:       "user:user-app",
			wantPath: orchestratorConfig.AppsDir().Join("user-app"),
		},
		{
			name:     "valid example id",
			in:       "examples:example-app",
			wantPath: examplesCommonDir.Join("example-app"),
		},
		{
			name:     "nested common example id",
			in:       "examples:nested/deep/example-app",
			wantPath: examplesCommonDir.Join("nested", "deep", "example-app"),
		},
		{
			name:     "nested platform example id wins over common",
			in:       "examples:nested/platform-example",
			wantPath: examplesUnoQDir.Join("nested", "platform-example"),
		},
		{
			name:     "absolute path to app",
			in:       tmp.Join("other-app").String(),
			wantPath: tmp.Join("other-app"),
		},
		{
			name:    "invalid id",
			in:      "invalid-id",
			wantErr: true,
		},
		{
			name:    "empty id",
			in:      "",
			wantErr: true,
		},
		{
			name:    "not existing system path raise an error",
			in:      "/non/existing/path",
			wantErr: true,
		},
		{
			name:     "not existing known location path DO NOT raise an error",
			in:       "examples:missing",
			wantPath: examplesCommonDir.Join("missing"),
		},
		{
			name:     "core-and-foundational should be processed",
			in:       "examples:core-and-foundational/example-app",
			wantPath: examplesBaseDir.Join("core-and-foundational", "example-app"),
		},
		{
			name:     "bricks should be processed",
			in:       "examples:bricks/example-app",
			wantPath: examplesBaseDir.Join("bricks", "example-app"),
		},
		{
			name:     "others_dir belongs to common, existing example",
			in:       "examples:others_dir/example-app",
			wantPath: examplesCommonDir.Join("others_dir", "example-app"),
		},
		{
			name:     "others_dir belongs to common, missing example",
			in:       "examples:others_dir/missing",
			wantPath: examplesCommonDir.Join("others_dir", "missing"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := idProvider.ParseID(tt.in)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantPath.String(), got.ToPath().String())
		})
	}
}
