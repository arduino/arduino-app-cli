package bricks

import (
	"os"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/store"
)

func TestBricksDetails________todo___________(t *testing.T) {
	basedir := paths.New("testdata", "assets", "0.4.8").String()
	service := setupTestService(t, basedir)
	testDataAssetsPath := paths.New(basedir)

	testDir := paths.New("testdata")
	t.Setenv("ARDUINO_APP_CLI__APPS_DIR", testDir.Join("apps").String())
	t.Setenv("ARDUINO_APP_CLI__CONFIG_DIR", testDir.Join("config").String())
	t.Setenv("ARDUINO_APP_CLI__DATA_DIR", testDir.String())

	cfg, err := config.NewFromEnv()
	require.NoError(t, err)
	idProvider := app.NewAppIDProvider(cfg)

	expectedVars := map[string]BrickVariable{
		"ARDUINO_DEVICE_ID": {
			DefaultValue: "",
			Description:  "Arduino Cloud Device ID",
			Required:     true,
		},
		"ARDUINO_SECRET": {
			DefaultValue: "",
			Description:  "Arduino Cloud Secret",
			Required:     true,
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
	expectedUsedByApps := []AppReference{
		{ID: "L2hvbWUvbWlya29jcm9idS9hcmR1aW5vX3Byb2plY3RzL2FyZHVpbm8tYXBwLWNsaS9pbnRlcm5hbC9vcmNoZXN0cmF0b3IvYnJpY2tzL3Rlc3RkYXRhL2V4YW1wbGVzL2Nsb3VkLWJsaW5r",
			Name: "Blinking LED from Arduino Cloud",
			Icon: "☁️",
		},
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
				UsedByApps:   expectedUsedByApps,
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
			result, err := service.BricksDetails(tc.brickID, idProvider, cfg)

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

	baseDir := paths.New("testdata", "assets", "0.4.8").String()
	service := setupTestService(t, baseDir)
	testDataAssetsPath := paths.New(baseDir)

	cfg, err := config.NewFromEnv()
	require.NoError(t, err)
	idProvider := app.NewAppIDProvider(cfg)

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
			result, err := service.BricksDetails(tc.brickID, idProvider, cfg)

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

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	bytes, err := os.ReadFile(path)
	require.NoError(t, err, "failed to read test file: %s", path)
	return bytes
}
