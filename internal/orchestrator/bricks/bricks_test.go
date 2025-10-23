package bricks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/store"
)

func TestBrickInstanceDetails(t *testing.T) {
	basaedir := getBasedir(t)
	service := setupTestService(t, basaedir)
	app := getTestApp(t, "testdata/object-detection", "object-detection")

	testCases := []struct {
		name                  string
		brickID               string
		wantErr               bool
		wantErrMsg            string
		expectedBrickInstance BrickInstance
	}{
		{
			name:    "Success - brick instance found in app",
			brickID: "arduino:arduino_cloud",
			wantErr: false,
			expectedBrickInstance: BrickInstance{
				ID:       "arduino:arduino_cloud",
				Name:     "Arduino Cloud",
				Author:   "Arduino",
				Category: "",
				Status:   "installed",
				Variables: map[string]string{
					"ARDUINO_DEVICE_ID": "<YOUR_DEVICE_ID>",
					"ARDUINO_SECRET":    "<YOUR_SECRET>",
				},
				ModelID: "",
			},
		},
		{
			name:       "Error - brick not found in index",
			brickID:    "arduino:non_existing_brick",
			wantErr:    true,
			wantErrMsg: "brick not found",
		},
		{
			name:       "Error - brick exists but is not in app",
			brickID:    "arduino:dbstorage_sqlstore",
			wantErr:    true,
			wantErrMsg: "not added in the app",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			brickInstance, err := service.AppBrickInstanceDetails(&app, tc.brickID)

			if tc.wantErr {
				require.Error(t, err)
				if tc.wantErrMsg != "" {
					require.Contains(t, err.Error(), tc.wantErrMsg)
				}

				assert.Equal(t, BrickInstance{}, brickInstance)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectedBrickInstance, brickInstance)
		})
	}
}
func TestBricksDetails1(t *testing.T) {
	basaedir := getBasedir(t)
	service := setupTestService(t, basaedir)
	testDataAssetsPath := paths.New(basaedir)

	expectedVars := map[string]BrickVariable{
		"ARDUINO_DEVICE_ID": {
			DefaultValue: "<YOUR_DEVICE_ID>",
			Description:  "Arduino Cloud Device ID",
			Required:     false,
		},
		"ARDUINO_SECRET": {
			DefaultValue: "<YOUR_SECRET>",
			Description:  "Arduino Cloud Secret",
			Required:     false,
		},
	}
	readmePath := testDataAssetsPath.Join("docs", "arduino", "arduino_cloud", "README.md")
	expectedReadmeBytes, err := os.ReadFile(readmePath.String())
	require.NoError(t, err, "Failed to read test readme file")
	expectedReadme := string(expectedReadmeBytes)
	expectedAPIPath := testDataAssetsPath.Join("api-docs", "arduino", "app_bricks", "arduino_cloud", "API.md").String()
	examplesBasePath := testDataAssetsPath.Join("examples", "arduino", "arduino_cloud")
	expectedExamples := []CodeExample{
		{Path: examplesBasePath.Join("1_led_blink.py").String()},
		{Path: examplesBasePath.Join("2_light_with_colors_monitor.py").String()},
		{Path: examplesBasePath.Join("3_light_with_colors_command.py").String()},
	}

	testCases := []struct {
		name           string
		brickID        string
		wantErr        bool
		wantErrMsg     string
		expectedResult BrickDetailsResult
	}{
		{
			name:    "Success - brick found",
			brickID: "arduino:arduino_cloud",
			wantErr: false,
			expectedResult: BrickDetailsResult{
				ID:           "arduino:arduino_cloud",
				Name:         "Arduino Cloud",
				Author:       "Arduino",
				Description:  "Connects to Arduino Cloud",
				Category:     "",
				Status:       "installed",
				Variables:    expectedVars,
				Readme:       expectedReadme,
				ApiDocsPath:  expectedAPIPath,
				CodeExamples: expectedExamples,
			},
		},
		{
			name:       "Error - brick not found",
			brickID:    "arduino:non_existing_brick",
			wantErr:    true,
			wantErrMsg: "brick not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := service.BricksDetails(tc.brickID)

			if tc.wantErr {
				require.Error(t, err)
				if tc.wantErrMsg != "" {
					require.Contains(t, err.Error(), tc.wantErrMsg)
				}
				assert.Equal(t, BrickDetailsResult{}, result)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestBricksDetails(t *testing.T) {

	basaedir := getBasedir(t)
	service := setupTestService(t, basaedir)
	testDataAssetsPath := paths.New(basaedir)

	testCases := []struct {
		name           string
		brickID        string
		wantErr        bool
		wantErrMsg     string
		expectedResult BrickDetailsResult
	}{
		{
			name:    "Success - brick found",
			brickID: "arduino:arduino_cloud",
			wantErr: false,
			expectedResult: BrickDetailsResult{
				ID:          "arduino:arduino_cloud",
				Name:        "Arduino Cloud",
				Author:      "Arduino",
				Description: "Connects to Arduino Cloud",
				Category:    "",
				Status:      "installed",
				Variables: map[string]BrickVariable{
					"ARDUINO_DEVICE_ID": {
						DefaultValue: "<YOUR_DEVICE_ID>",
						Description:  "Arduino Cloud Device ID",
						Required:     false,
					},
					"ARDUINO_SECRET": {
						DefaultValue: "<YOUR_SECRET>",
						Description:  "Arduino Cloud Secret",
						Required:     false,
					},
				},
				Readme: string(mustReadFile(t, testDataAssetsPath.Join(
					"docs", "arduino", "arduino_cloud", "README.md",
				).String())),
				ApiDocsPath: testDataAssetsPath.Join(
					"api-docs", "arduino", "app_bricks", "arduino_cloud", "API.md",
				).String(),
				CodeExamples: []CodeExample{
					{Path: testDataAssetsPath.Join("examples", "arduino", "arduino_cloud", "1_led_blink.py").String()},
					{Path: testDataAssetsPath.Join("examples", "arduino", "arduino_cloud", "2_light_with_colors_monitor.py").String()},
					{Path: testDataAssetsPath.Join("examples", "arduino", "arduino_cloud", "3_light_with_colors_command.py").String()},
				},
			},
		},
		{
			name:       "Error - brick not found",
			brickID:    "arduino:non_existing_brick",
			wantErr:    true,
			wantErrMsg: "brick not found",
		},
		{
			name:    "Success - brick with nil examples",
			brickID: "arduino:streamlit_ui",
			wantErr: false,
			expectedResult: BrickDetailsResult{
				ID:          "arduino:streamlit_ui",
				Name:        "WebUI - Streamlit",
				Author:      "Arduino",
				Description: "A simplified user interface based on Streamlit and Python.",
				Category:    "ui",
				Status:      "installed",
				Variables:   map[string]BrickVariable{},
				Readme: string(mustReadFile(t, testDataAssetsPath.Join(
					"docs", "arduino", "streamlit_ui", "README.md",
				).String())),
				ApiDocsPath: testDataAssetsPath.Join(
					"api-docs", "arduino", "app_bricks", "streamlit_ui", "API.md",
				).String(),
				CodeExamples: []CodeExample{},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := service.BricksDetails(tc.brickID)

			if tc.wantErr {
				// --- Error Case ---
				require.Error(t, err)
				if tc.wantErrMsg != "" {
					require.Contains(t, err.Error(), tc.wantErrMsg)
				}
				assert.Equal(t, BrickDetailsResult{}, result)
				return
			}

			// --- Success Case ---
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func setupTestService(t *testing.T, baseDir string) *Service {
	store := store.NewStaticStore(baseDir)

	bricksIndex, err := bricksindex.GenerateBricksIndexFromFile(paths.New(baseDir))
	require.NoError(t, err)

	service := NewService(nil, bricksIndex, store)
	return service
}

func getBasedir(t *testing.T) string {
	cfg, err := config.NewFromEnv()
	require.NoError(t, err)
	baseDir := paths.New("../../e2e/daemon/testdata", "assets", cfg.RunnerVersion).String()
	return baseDir
}
func getTestApp(t *testing.T, appPath, appName string) app.ArduinoApp {
	app, err := app.Load(appPath)
	assert.NoError(t, err)
	assert.NotEmpty(t, app)
	assert.NotNil(t, app.MainPythonFile)
	assert.Equal(t, f.Must(filepath.Abs("testdata/"+appName+"/python/main.py")), app.MainPythonFile.String())
	// in case you want to test sketch based apps too
	// assert.NotNil(t, app.MainSketchPath)
	// assert.Equal(t, f.Must(filepath.Abs("testdata/"+appName+"/sketch")), app.MainSketchPath.String())
	return app
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	bytes, err := os.ReadFile(path)
	require.NoError(t, err, "failed to read test file: %s", path)
	return bytes
}
