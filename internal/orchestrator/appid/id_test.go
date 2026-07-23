// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package appid

import (
	"encoding/base64"
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
	examplesBoardDir := orchestratorConfig.DataDir().Join("examples/inspirational", "platform_unoq")
	examplesCommonDir := orchestratorConfig.DataDir().Join("examples/inspirational", "common")
	require.NoError(t, orchestratorConfig.AppsDir().Join("user-app").MkdirAll())
	require.NoError(t, examplesCommonDir.Join("example-app").MkdirAll())
	require.NoError(t, examplesBoardDir.Join("special-example-app").MkdirAll())
	require.NoError(t, examplesCommonDir.Join("duplicated-example-app").MkdirAll())
	require.NoError(t, examplesBoardDir.Join("duplicated-example-app").MkdirAll())
	require.NoError(t, examplesCommonDir.Join("nested", "deep", "example-app").MkdirAll())
	require.NoError(t, examplesBoardDir.Join("nested", "platform-example").MkdirAll())
	require.NoError(t, tmp.Join("other-app").MkdirAll())

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
			in:   examplesBoardDir.Join("duplicated-example-app"),
			want: f.Must(idProvider.ParseID("examples:duplicated-example-app")),
		},
		{
			name: "platform specific valid example id",
			in:   examplesBoardDir.Join("special-example-app"),
			want: f.Must(idProvider.ParseID("examples:special-example-app")),
		},
		{
			name: "nested common example id",
			in:   examplesCommonDir.Join("nested", "deep", "example-app"),
			want: f.Must(idProvider.ParseID("examples:nested/deep/example-app")),
		},
		{
			name: "nested platform example id",
			in:   examplesBoardDir.Join("nested", "platform-example"),
			want: f.Must(idProvider.ParseID("examples:nested/platform-example")),
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
	examplesBoardDir := orchestratorConfig.DataDir().Join("examples/inspirational", "platform_unoq")
	examplesCommonDir := orchestratorConfig.DataDir().Join("examples/inspirational", "common")
	require.NoError(t, examplesCommonDir.Join("example-app").MkdirAll())
	require.NoError(t, examplesCommonDir.Join("nested", "deep", "example-app").MkdirAll())
	require.NoError(t, examplesBoardDir.Join("nested", "platform-example").MkdirAll())

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
			wantPath: examplesBoardDir.Join("nested", "platform-example"),
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
			name:    "not existing path",
			in:      "/non/existing/path",
			wantErr: true,
		},
		{
			name:    "not_existing_selector:my_example",
			in:      "not_existing_selector:my_example",
			wantErr: true,
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

func TestNewExamplesLayout(t *testing.T) {
	tmp := paths.New(t.TempDir())
	t.Setenv("ARDUINO_APP_CLI__APPS_DIR", tmp.Join("apps").String())
	t.Setenv("ARDUINO_APP_CLI__DATA_DIR", tmp.Join("data").String())

	orchestratorConfig, err := config.NewFromEnv()
	idProvider := NewAppProvider(orchestratorConfig, unoQPlatform)
	require.NoError(t, err)
	// Legacy dir examples: refers examples/inspirational
	examplesBoardUno := orchestratorConfig.DataDir().Join("examples/inspirational", "platform_unoq")
	examplesBoardVentuno := orchestratorConfig.DataDir().Join("examples/inspirational", "platform_ventunoq")
	examplesCommon := orchestratorConfig.DataDir().Join("examples/inspirational", "common")
	// New examples
	examplesCoreAndFoundational := orchestratorConfig.DataDir().Join("examples/core-and-foundational")
	examplesBricks := orchestratorConfig.DataDir().Join("examples/bricks")
	examplesOther := orchestratorConfig.DataDir().Join("examples/other")

	// Create the full examples structure for testing all cases
	require.NoError(t, examplesBoardUno.Join("specific_example").MkdirAll())
	require.NoError(t, examplesBoardVentuno.Join("specific_example").MkdirAll())
	require.NoError(t, examplesBoardVentuno.Join("ventuno_specific_example").MkdirAll())
	require.NoError(t, examplesCommon.Join("common_example").MkdirAll())
	require.NoError(t, examplesCoreAndFoundational.Join("coreEx0").MkdirAll())
	require.NoError(t, examplesCoreAndFoundational.Join("coreDir1/coreEx10").MkdirAll())
	require.NoError(t, examplesCoreAndFoundational.Join("coreDir1/coreEx11").MkdirAll())
	require.NoError(t, examplesCoreAndFoundational.Join("coreDir2/coreEx21").MkdirAll())
	require.NoError(t, examplesBricks.Join("arduino").Join("brick1").Join("brickEx10").MkdirAll())
	require.NoError(t, examplesBricks.Join("arduino").Join("brick1").Join("brickEx11").MkdirAll())
	require.NoError(t, examplesBricks.Join("arduino").Join("brick2").Join("brickEx20").MkdirAll())
	require.NoError(t, examplesOther.Join("other1").Join("otherEx11").MkdirAll())
	require.NoError(t, examplesOther.Join("other2").MkdirAll())

	tests := []struct {
		name         string
		id           string
		path         *paths.Path
		idNotFound   bool
		pathNotFound bool
	}{
		{
			name: "legacy, get unoq example",
			id:   "examples:specific_example",
			path: examplesBoardUno.Join("specific_example"),
		},
		{
			name:       "legacy, get unoq example defined in the ventuno path",
			id:         "examples:ventuno_specific_example",
			path:       examplesBoardVentuno.Join("ventuno_specific_example"),
			idNotFound: true,
		},
		{
			name: "legacy, get common example",
			id:   "examples:common_example",
			path: examplesCommon.Join("common_example"),
		},
		{
			name:       "legacy, coreEx10 is a core example. examples: should not look outside inspirational",
			id:         "examples:coreEx10",
			idNotFound: true,
			path:       examplesCoreAndFoundational.Join("coreDir1/coreEx10"),
		},
		{
			name:       "legacy, brickEx10 is a brick example. examples: should not look outside inspirational",
			id:         "examples:brickEx10",
			idNotFound: true,
			path:       examplesBricks.Join("arduino").Join("brick1").Join("brickEx10"),
		},
		{
			name:       "legacy, otherEx11 is another location example. examples: should not look outside inspirational",
			id:         "examples:otherEx11",
			idNotFound: true,
			path:       examplesOther.Join("other1").Join("otherEx11"),
		},
		{
			name:       "inspirational is reserved, need to avoid to retrieve non platform examples",
			id:         "examples:inspirational/platform_unoq/specific_example",
			idNotFound: true,
			path:       examplesBoardUno.Join("specific_example"),
		},
		{
			name: "get an example from core-and-foundational",
			id:   "examples:core-and-foundational/coreEx0",
			path: examplesCoreAndFoundational.Join("coreEx0"),
		},
		{
			name: "get a nested example from core-and-foundational",
			id:   "examples:core-and-foundational/coreDir1/coreEx10",
			path: examplesCoreAndFoundational.Join("coreDir1/coreEx10"),
		},
		{
			name:         "core-and-foundational missing id example",
			id:           "examples:core-and-foundational/common_example",
			idNotFound:   true,
			path:         examplesCoreAndFoundational.Join("common_example"),
			pathNotFound: true,
		},
		{
			name:         "core-and-foundational missing path example",
			id:           "examples:core-and-foundational/missing_example",
			path:         examplesCoreAndFoundational.Join("missing_example"),
			idNotFound:   true,
			pathNotFound: true,
		},
		{
			name: "get an example from bricks",
			id:   "examples:bricks/arduino/brick1/brickEx10",
			path: examplesBricks.Join("arduino").Join("brick1").Join("brickEx10"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			/* Test path from id */
			got, err := idProvider.ParseID(tt.id)
			if tt.idNotFound {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.path.String(), got.ToPath().String())
			}
			if tt.idNotFound == false && tt.pathNotFound == false {
				assert.Equal(t, tt.path.String(), got.ToPath().String())
			}

			/* Test id from path */
			got, err = idProvider.IDFromPath(tt.path)
			if tt.pathNotFound {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

			}
			if tt.idNotFound == false && tt.pathNotFound == false {
				assert.Equal(t, base64.RawURLEncoding.EncodeToString([]byte(tt.id)), got.encodedID)
			}

		})
	}

}
