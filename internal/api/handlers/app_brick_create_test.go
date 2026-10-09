// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

func TestGenerateBrickID(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedID   string
		errorMessage string
	}{
		{
			name:       "simple name",
			input:      "MyBrick",
			expectedID: "mybrick",
		},
		{
			name:       "name with spaces",
			input:      "My Awesome Brick",
			expectedID: "my_awesome_brick",
		},
		{
			name:       "name with hyphens",
			input:      "my-brick",
			expectedID: "my_brick",
		},
		{
			name:       "name with numbers",
			input:      "Brick123",
			expectedID: "brick123",
		},
		{
			name:       "name with mixed case and special chars",
			input:      "My-Awesome_Brick",
			expectedID: "my_awesome_brick",
		},
		{
			name:       "name with leading/trailing spaces",
			input:      "  MyBrick  ",
			expectedID: "mybrick",
		},
		{
			name:       "single character name",
			input:      "A",
			expectedID: "a",
		},
		{
			name:         "only special characters",
			input:        "---___",
			expectedID:   "",
			errorMessage: "brick name must contain at least one alphanumeric character",
		},
		{
			name:       "mixed valid and underscore",
			input:      "my_brick",
			expectedID: "my_brick",
		},
		{
			name:       "converts to lowercase",
			input:      "MyBrick",
			expectedID: "mybrick",
		},
		{
			name:       "replaces spaces with underscores",
			input:      "my brick",
			expectedID: "my_brick",
		},
		{
			name:       "replaces multiple spaces with single underscore",
			input:      "my   brick",
			expectedID: "my_brick",
		},
		{
			name:       "replaces hyphens with underscores",
			input:      "my-brick",
			expectedID: "my_brick",
		},
		{
			name:       "trims leading underscores",
			input:      "___mybrick",
			expectedID: "mybrick",
		},
		{
			name:       "trims trailing underscores",
			input:      "mybrick___",
			expectedID: "mybrick",
		},
		{
			name:       "trims both sides",
			input:      "___mybrick___",
			expectedID: "mybrick",
		},
		{
			name:       "keeps internal alphanumerics and underscores",
			input:      "my_brick_123",
			expectedID: "my_brick_123",
		},
		{
			name:       "alphanumeric",
			input:      "abc123",
			expectedID: "abc123",
		},
		{
			name:       "uppercase",
			input:      "MYBRICK",
			expectedID: "mybrick",
		},
		{
			name:       "lowercase",
			input:      "mybrick",
			expectedID: "mybrick",
		},
		{
			name:       "numbers only",
			input:      "123456",
			expectedID: "123456",
		},
		{
			name:       "with special chars that get converted",
			input:      "my@brick#test",
			expectedID: "my_brick_test",
		},
		{
			name:         "name with dot character - should error",
			input:        "my.brick",
			expectedID:   "",
			errorMessage: "brick name cannot contain '.' character",
		},
		{
			name:         "name with multiple dots - should error",
			input:        "my.awesome.brick",
			expectedID:   "",
			errorMessage: "brick name cannot contain '.' character",
		},
		{
			name:         "name with dot at the beginning - should error",
			input:        ".mybrick",
			expectedID:   "",
			errorMessage: "brick name cannot contain '.' character",
		},
		{
			name:         "complex valid name with dot",
			input:        "My Awesome Brick v2.0",
			expectedID:   "",
			errorMessage: "brick name cannot contain '.' character",
		},
		{
			name:         "name with colon character - should error",
			input:        "my:brick",
			expectedID:   "",
			errorMessage: "brick name cannot contain ':' character",
		},
		{
			name:         "name with multiple colons - should error",
			input:        "my:awesome:brick",
			expectedID:   "",
			errorMessage: "brick name cannot contain ':' character",
		},
		{
			name:         "name with colon at the end - should error",
			input:        "mybrick:",
			expectedID:   "",
			errorMessage: "brick name cannot contain ':' character",
		},
		{
			name:         "rejects dot before colon",
			input:        "test.name:brick",
			expectedID:   "",
			errorMessage: "brick name cannot contain '.' character",
		},
		{
			name:         "name with both dot and colon - should error on dot first",
			input:        "my.brick:test",
			expectedID:   "",
			errorMessage: "brick name cannot contain '.' character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := generateBrickID(tt.input)

			if tt.errorMessage != "" {
				require.Error(t, err, "expected error but got none")
				require.Equal(t, tt.errorMessage, err.Error(), "error message mismatch")
			} else {
				require.NoError(t, err, "expected no error but got: %v", err)
				require.Equal(t, tt.expectedID, id, "generated ID mismatch")
			}
		})
	}
}

func TestHandleAppLocalBrickCreate(t *testing.T) {
	tmpDir := paths.New(t.TempDir())
	t.Setenv("ARDUINO_APP_CLI__APPS_DIR", tmpDir.Join("apps").String())
	t.Setenv("ARDUINO_APP_CLI__CONFIG_DIR", tmpDir.Join("config").String())
	t.Setenv("ARDUINO_APP_CLI__DATA_DIR", tmpDir.Join("data").String())
	cfg, err := config.NewFromEnv()
	require.NoError(t, err)

	idProvider := appid.NewAppProvider(cfg, platform.Platform{BoardName: "unoq"})

	testAppDir := cfg.AppsDir().Join("test-app")
	srcApp := paths.New("../../orchestrator/app/testdata/AppWithLocalBricks")
	require.NoError(t, srcApp.CopyDirTo(testAppDir))

	// In AppWithLocalBricks, folder is "bricks/my-first-brick".
	// Rename id in brick_config.yaml to "my_test" without renaming the folder.
	configPath := testAppDir.Join("bricks", "my-first-brick", "brick_config.yaml")
	require.NoError(t, configPath.WriteFile([]byte("id: my_test\nname: My Test\n")))

	appID, err := idProvider.IDFromPath(testAppDir)
	require.NoError(t, err)

	handler := HandleAppLocalBrickCreate(idProvider)

	t.Run("fails when local brick already exists with the same ID even if folder differs", func(t *testing.T) {
		body := bytes.NewBufferString(`{"name": "my_test"}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/apps/"+appID.String()+"/bricks", body)
		req.SetPathValue("appID", appID.String())
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusConflict, w.Code)
		var errResp models.ErrorResponse
		require.NoError(t, json.NewDecoder(w.Body).Decode(&errResp))
		require.Equal(t, "a brick with the same id 'my_test' already exists", errResp.Details)
		require.False(t, testAppDir.Join("bricks", "my_test").Exist())
	})

	t.Run("successfully creates local brick when ID is unique", func(t *testing.T) {
		body := bytes.NewBufferString(`{"name": "unique_brick"}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/apps/"+appID.String()+"/bricks", body)
		req.SetPathValue("appID", appID.String())
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code)
		var resp AppLocalBrickCreateResponse
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		require.Equal(t, "unique_brick", resp.ID)
		require.True(t, testAppDir.Join("bricks", "unique_brick").Exist())
	})

	t.Run("fails when brick name is empty", func(t *testing.T) {
		body := bytes.NewBufferString(`{"name": ""}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/apps/"+appID.String()+"/bricks", body)
		req.SetPathValue("appID", appID.String())
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("fails when brick name contains invalid characters", func(t *testing.T) {
		body := bytes.NewBufferString(`{"name": "invalid.brick"}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/apps/"+appID.String()+"/bricks", body)
		req.SetPathValue("appID", appID.String())
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}
