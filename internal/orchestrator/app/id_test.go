// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package app

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func TestNewIDFromPath(t *testing.T) {
	tmp := paths.New(t.TempDir())
	t.Setenv("ARDUINO_APP_CLI__APPS_DIR", tmp.Join("apps").String())
	t.Setenv("ARDUINO_APP_CLI__DATA_DIR", tmp.Join("data").String())

	orchestratorConfig, err := config.NewFromEnv()
	require.NoError(t, err)
	require.NoError(t, orchestratorConfig.AppsDir().Join("user-app").MkdirAll())
	require.NoError(t, orchestratorConfig.ExamplesDir().Join("example-app").MkdirAll())
	require.NoError(t, tmp.Join("other-app").MkdirAll())

	idProvider := NewAppIDProvider(orchestratorConfig)

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
			in:   orchestratorConfig.ExamplesDir().Join("example-app"),
			want: f.Must(idProvider.ParseID("examples:example-app")),
		},
		{
			name: "valid absolute path",
			in:   tmp.Join("other-app"),
			want: f.Must(idProvider.IDFromPath(tmp.Join("other-app"))),
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

	idProvider := NewAppIDProvider(orchestratorConfig)

	tests := []struct {
		name    string
		in      string
		want    ID
		wantErr bool
	}{
		{
			name: "valid user id",
			in:   "user:user-app",
			want: f.Must(idProvider.ParseID("user:user-app")),
		},
		{
			name: "valid example id",
			in:   "examples:example-app",
			want: f.Must(idProvider.ParseID("examples:example-app")),
		},
		{
			name: "absolute path to app",
			in:   tmp.Join("other-app").String(),
			want: f.Must(idProvider.IDFromPath(tmp.Join("other-app"))),
		},
		{
			name:    "invalid id",
			in:      "invalid-id",
			want:    ID{},
			wantErr: true,
		},
		{
			name:    "empty id",
			in:      "",
			want:    ID{},
			wantErr: true,
		},
		{
			name:    "not existing path",
			in:      "/non/existing/path",
			want:    ID{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := idProvider.ParseID(tt.in)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
