// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package bricks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

func TestBrickCreate(t *testing.T) {
	bricksIndex, err := bricksindex.Load(platform.GetPlatform(nil), paths.New("testdata"))
	require.Nil(t, err)
	brickService := NewService(nil, bricksIndex)

	t.Run("fails if brick id does not exist", func(t *testing.T) {
		err = brickService.BrickCreate(BrickCreateUpdateRequest{ID: "not-existing-id"}, f.Must(app.Load(paths.New("testdata/dummy-app"))))
		require.Error(t, err)
		require.Equal(t, "brick \"not-existing-id\" not found", err.Error())
	})

	t.Run("fails if the requestes variable is not present in the brick definition", func(t *testing.T) {
		req := BrickCreateUpdateRequest{ID: "arduino:arduino_cloud", Variables: map[string]string{
			"NON_EXISTING_VARIABLE": "some-value",
		}}
		err = brickService.BrickCreate(req, f.Must(app.Load(paths.New("testdata/dummy-app"))))
		require.Error(t, err)
		require.Equal(t, "variable \"NON_EXISTING_VARIABLE\" does not exist on brick \"arduino:arduino_cloud\"", err.Error())
	})

	t.Run("fails if a required variable is set empty", func(t *testing.T) {
		req := BrickCreateUpdateRequest{ID: "arduino:arduino_cloud", Variables: map[string]string{
			"ARDUINO_DEVICE_ID": "",
			"ARDUINO_SECRET":    "a-secret-a",
		}}
		err = brickService.BrickCreate(req, f.Must(app.Load(paths.New("testdata/dummy-app"))))
		require.Error(t, err)
		require.Equal(t, "required variable \"ARDUINO_DEVICE_ID\" cannot be empty", err.Error())
	})

	t.Run("do not fail if a mandatory variable is not present", func(t *testing.T) {
		tempDummyApp := paths.New("testdata/dummy-app.temp")
		err := tempDummyApp.RemoveAll()
		require.Nil(t, err)
		require.Nil(t, paths.New("testdata/dummy-app").CopyDirTo(tempDummyApp))

		req := BrickCreateUpdateRequest{ID: "arduino:arduino_cloud", Variables: map[string]string{
			"ARDUINO_SECRET": "a-secret-a",
		}}
		err = brickService.BrickCreate(req, f.Must(app.Load(tempDummyApp)))
		require.NoError(t, err)

		after, err := app.Load(tempDummyApp)
		require.Nil(t, err)
		require.Len(t, after.Descriptor.Bricks, 1)
		require.Equal(t, "arduino:arduino_cloud", after.Descriptor.Bricks[0].ID)
		require.Equal(t, "", after.Descriptor.Bricks[0].Variables["ARDUINO_DEVICE_ID"])
		require.Equal(t, "a-secret-a", after.Descriptor.Bricks[0].Variables["ARDUINO_SECRET"])
	})

	t.Run("the brick is added if it does not exist in the app", func(t *testing.T) {
		tempDummyApp := paths.New("testdata/dummy-app.temp")
		err := tempDummyApp.RemoveAll()
		require.Nil(t, err)
		require.Nil(t, paths.New("testdata/dummy-app").CopyDirTo(tempDummyApp))

		req := BrickCreateUpdateRequest{ID: "arduino:dbstorage_sqlstore"}
		err = brickService.BrickCreate(req, f.Must(app.Load(tempDummyApp)))
		require.Nil(t, err)
		after, err := app.Load(tempDummyApp)
		require.Nil(t, err)
		require.Len(t, after.Descriptor.Bricks, 2)
		require.Equal(t, "arduino:dbstorage_sqlstore", after.Descriptor.Bricks[1].ID)
	})

	t.Run("the variables of a brick are updated", func(t *testing.T) {
		tempDummyApp := paths.New("testdata/dummy-app.brick-override.temp")
		err := tempDummyApp.RemoveAll()
		require.Nil(t, err)
		err = paths.New("testdata/dummy-app").CopyDirTo(tempDummyApp)
		require.Nil(t, err)
		bricksIndex, err := bricksindex.Load(platform.GetPlatform(nil), paths.New("testdata"))
		require.Nil(t, err)
		brickService := NewService(nil, bricksIndex)

		deviceID := "this-is-a-device-id"
		secret := "this-is-a-secret"
		req := BrickCreateUpdateRequest{
			ID: "arduino:arduino_cloud",
			Variables: map[string]string{
				"ARDUINO_DEVICE_ID": deviceID,
				"ARDUINO_SECRET":    secret,
			},
		}

		err = brickService.BrickCreate(req, f.Must(app.Load(tempDummyApp)))
		require.Nil(t, err)

		after, err := app.Load(tempDummyApp)
		require.Nil(t, err)
		require.Len(t, after.Descriptor.Bricks, 1)
		require.Equal(t, "arduino:arduino_cloud", after.Descriptor.Bricks[0].ID)
		require.Equal(t, deviceID, after.Descriptor.Bricks[0].Variables["ARDUINO_DEVICE_ID"])
		require.Equal(t, secret, after.Descriptor.Bricks[0].Variables["ARDUINO_SECRET"])
	})
}

func TestUpdateBrick(t *testing.T) {
	bricksIndex, err := bricksindex.Load(platform.GetPlatform(nil), paths.New("testdata"))
	require.Nil(t, err)
	brickService := NewService(nil, bricksIndex)

	t.Run("fails if brick id does not exist into brick index", func(t *testing.T) {
		err = brickService.BrickUpdate(BrickCreateUpdateRequest{ID: "not-existing-id"}, f.Must(app.Load(paths.New("testdata/dummy-app"))))
		require.Error(t, err)
		require.Equal(t, "brick \"not-existing-id\" not found into the brick index", err.Error())
	})

	t.Run("fails if brick is present into the index but not in the app ", func(t *testing.T) {
		err = brickService.BrickUpdate(BrickCreateUpdateRequest{ID: "arduino:dbstorage_sqlstore"}, f.Must(app.Load(paths.New("testdata/dummy-app"))))
		require.Error(t, err)
		require.Equal(t, "brick \"arduino:dbstorage_sqlstore\" not found into the bricks of the app", err.Error())
	})

	t.Run("fails if the updated variable is not present in the brick definition", func(t *testing.T) {
		req := BrickCreateUpdateRequest{ID: "arduino:arduino_cloud", Variables: map[string]string{
			"NON_EXISTING_VARIABLE": "some-value",
		}}
		err = brickService.BrickUpdate(req, f.Must(app.Load(paths.New("testdata/dummy-app"))))
		require.Error(t, err)
		require.Equal(t, "variable \"NON_EXISTING_VARIABLE\" does not exist on brick \"arduino:arduino_cloud\"", err.Error())
	})

	// TODO: allow to set an empty "" variable
	t.Run("fails if a required variable is set empty", func(t *testing.T) {
		req := BrickCreateUpdateRequest{ID: "arduino:arduino_cloud", Variables: map[string]string{
			"ARDUINO_DEVICE_ID": "",
			"ARDUINO_SECRET":    "a-secret-a",
		}}
		err = brickService.BrickUpdate(req, f.Must(app.Load(paths.New("testdata/dummy-app"))))
		require.Error(t, err)
		require.Equal(t, "required variable \"ARDUINO_DEVICE_ID\" cannot be empty", err.Error())
	})

	t.Run("allow updating only one mandatory variable among two", func(t *testing.T) {
		tempDummyApp := paths.New("testdata/dummy-app.temp")
		err := tempDummyApp.RemoveAll()
		require.Nil(t, err)
		require.Nil(t, paths.New("testdata/dummy-app").CopyDirTo(tempDummyApp))

		req := BrickCreateUpdateRequest{ID: "arduino:arduino_cloud", Variables: map[string]string{
			"ARDUINO_SECRET": "a-secret-a",
		}}
		err = brickService.BrickUpdate(req, f.Must(app.Load(tempDummyApp)))
		require.NoError(t, err)

		after, err := app.Load(tempDummyApp)
		require.Nil(t, err)
		require.Len(t, after.Descriptor.Bricks, 1)
		require.Equal(t, "arduino:arduino_cloud", after.Descriptor.Bricks[0].ID)
		require.Equal(t, "", after.Descriptor.Bricks[0].Variables["ARDUINO_DEVICE_ID"])
		require.Equal(t, "a-secret-a", after.Descriptor.Bricks[0].Variables["ARDUINO_SECRET"])
	})

	t.Run("update a single variables of a brick correctly", func(t *testing.T) {
		tempDummyApp := paths.New("testdata/dummy-app.temp")
		require.Nil(t, tempDummyApp.RemoveAll())
		require.Nil(t, paths.New("testdata/dummy-app").CopyDirTo(tempDummyApp))
		bricksIndex, err := bricksindex.Load(platform.GetPlatform(nil), paths.New("testdata"))
		require.Nil(t, err)
		brickService := NewService(nil, bricksIndex)

		deviceID := "updated-device-id"
		secret := "updated-secret"
		req := BrickCreateUpdateRequest{
			ID: "arduino:arduino_cloud",
			Variables: map[string]string{
				"ARDUINO_DEVICE_ID": deviceID,
				"ARDUINO_SECRET":    secret,
			},
		}

		err = brickService.BrickUpdate(req, f.Must(app.Load(tempDummyApp)))
		require.Nil(t, err)

		after, err := app.Load(tempDummyApp)
		require.Nil(t, err)
		require.Len(t, after.Descriptor.Bricks, 1)
		require.Equal(t, "arduino:arduino_cloud", after.Descriptor.Bricks[0].ID)
		require.Equal(t, deviceID, after.Descriptor.Bricks[0].Variables["ARDUINO_DEVICE_ID"])
		require.Equal(t, secret, after.Descriptor.Bricks[0].Variables["ARDUINO_SECRET"])
	})

	t.Run("update a single variable correctly", func(t *testing.T) {
		tempDummyApp := paths.New("testdata/dummy-app-for-update.temp")
		require.Nil(t, tempDummyApp.RemoveAll())
		require.Nil(t, paths.New("testdata/dummy-app-for-update").CopyDirTo(tempDummyApp))
		bricksIndex, err := bricksindex.Load(platform.GetPlatform(nil), paths.New("testdata"))
		require.Nil(t, err)
		brickService := NewService(nil, bricksIndex)

		secret := "updated-the-secret"
		req := BrickCreateUpdateRequest{
			ID: "arduino:arduino_cloud",
			Variables: map[string]string{
				// the ARDUINO_DEVICE_ID is already configured int the app.yaml
				"ARDUINO_SECRET": secret,
			},
		}

		err = brickService.BrickUpdate(req, f.Must(app.Load(tempDummyApp)))
		require.Nil(t, err)

		after, err := app.Load(tempDummyApp)
		require.Nil(t, err)
		require.Len(t, after.Descriptor.Bricks, 1)
		require.Equal(t, "arduino:arduino_cloud", after.Descriptor.Bricks[0].ID)
		require.Equal(t, "i-am-a-device-id", after.Descriptor.Bricks[0].Variables["ARDUINO_DEVICE_ID"])
		require.Equal(t, secret, after.Descriptor.Bricks[0].Variables["ARDUINO_SECRET"])
	})

	t.Run("update a custom model definition in a brick", func(t *testing.T) {
		tempDummyApp := paths.New("testdata/dummy-app-for-model.temp")
		require.Nil(t, tempDummyApp.RemoveAll())
		require.Nil(t, paths.New("testdata/dummy-app-for-model").CopyDirTo(tempDummyApp))
		bricksIndex, err := bricksindex.Load(platform.GetPlatform(nil), paths.New("testdata"))
		require.NoError(t, err)
		modelsIndex, err := modelsindex.Load(platform.GetPlatform(nil), paths.New("testdata"), paths.New("not_exixsting_path"))
		require.NoError(t, err)
		brickService := NewService(modelsIndex, bricksIndex)

		modelPath := "/home/arduino/.arduino-bricks/ei-model-123-1/model.eim"
		modelId := "ei-model-123-1"
		brickId := "arduino:brick-with-custom-model"
		req := BrickCreateUpdateRequest{
			ID:    brickId,
			Model: f.Ptr(modelId),
			Variables: map[string]string{
				"EI_OBJ_DETECTION_MODEL": modelId,
				"CUSTOM_MODEL_PATH":      modelPath,
			},
		}

		err = brickService.BrickUpdate(req, f.Must(app.Load(tempDummyApp)))
		require.Nil(t, err)

		after, err := app.Load(tempDummyApp)
		require.Nil(t, err)
		require.Len(t, after.Descriptor.Bricks, 1)
		require.Equal(t, brickId, after.Descriptor.Bricks[0].ID)
		require.Equal(t, modelId, after.Descriptor.Bricks[0].Model)
		require.Equal(t, modelId, after.Descriptor.Bricks[0].Variables["EI_OBJ_DETECTION_MODEL"])
		require.Equal(t, modelPath, after.Descriptor.Bricks[0].Variables["CUSTOM_MODEL_PATH"])
	})

}

func TestGetBrickInstanceVariableDetails(t *testing.T) {
	tests := []struct {
		name                    string
		brick                   *bricksindex.Brick
		userVariables           map[string]string
		expectedConfigVariables []BrickConfigVariable
		expectedVariableMap     map[string]string
	}{
		{
			name: "variable is present in the map",
			brick: &bricksindex.Brick{
				Variables: []bricksindex.BrickVariable{
					{Name: "VAR1", Description: "desc"},
				},
			},
			userVariables: map[string]string{"VAR1": "value1"},
			expectedConfigVariables: []BrickConfigVariable{
				{Name: "VAR1", Value: "value1", Description: "desc", Required: true},
			},
			expectedVariableMap: map[string]string{"VAR1": "value1"},
		},
		{
			name: "variable not present in the map",
			brick: &bricksindex.Brick{
				Variables: []bricksindex.BrickVariable{
					{Name: "VAR1", Description: "desc"},
				},
			},
			userVariables: map[string]string{},
			expectedConfigVariables: []BrickConfigVariable{
				{Name: "VAR1", Value: "", Description: "desc", Required: true},
			},
			expectedVariableMap: map[string]string{"VAR1": ""},
		},
		{
			name: "variable with default value",
			brick: &bricksindex.Brick{
				Variables: []bricksindex.BrickVariable{
					{Name: "VAR1", DefaultValue: "default", Description: "desc"},
				},
			},
			userVariables: map[string]string{},
			expectedConfigVariables: []BrickConfigVariable{
				{Name: "VAR1", Value: "default", Description: "desc", Required: false},
			},
			expectedVariableMap: map[string]string{"VAR1": "default"},
		},
		{
			name: "multiple variables",
			brick: &bricksindex.Brick{
				Variables: []bricksindex.BrickVariable{
					{Name: "VAR1", Description: "desc1"},
					{Name: "VAR2", DefaultValue: "def2", Description: "desc2"},
				},
			},
			userVariables: map[string]string{"VAR1": "v1"},
			expectedConfigVariables: []BrickConfigVariable{
				{Name: "VAR1", Value: "v1", Description: "desc1", Required: true},
				{Name: "VAR2", Value: "def2", Description: "desc2", Required: false},
			},
			expectedVariableMap: map[string]string{"VAR1": "v1", "VAR2": "def2"},
		},
		{
			name:                    "no variables",
			brick:                   &bricksindex.Brick{Variables: []bricksindex.BrickVariable{}},
			userVariables:           map[string]string{},
			expectedConfigVariables: []BrickConfigVariable{},
			expectedVariableMap:     map[string]string{},
		},
		{
			name: "hidden variables",
			brick: &bricksindex.Brick{Variables: []bricksindex.BrickVariable{
				{Name: "HIDDEN_VAR", DefaultValue: "i-am-hidden", Description: "a-hidden-variable", Hidden: true},
				{Name: "VISIBLE_VAR", DefaultValue: "i-am-visible", Description: "a-visible-variable", Hidden: false},
				{Name: "VISIBLE_VAR_WITH_MISSING", DefaultValue: "i-am-visible-if-missing-hidden", Description: "a-visible-variable"},
			}},
			userVariables: map[string]string{},
			expectedConfigVariables: []BrickConfigVariable{
				{Name: "VISIBLE_VAR", Value: "i-am-visible", Description: "a-visible-variable", Required: false},
				{Name: "VISIBLE_VAR_WITH_MISSING", Value: "i-am-visible-if-missing-hidden", Description: "a-visible-variable", Required: false},
			},
			expectedVariableMap: map[string]string{"VISIBLE_VAR": "i-am-visible", "VISIBLE_VAR_WITH_MISSING": "i-am-visible-if-missing-hidden"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualVariableMap, actualConfigVariables := getInstanceBrickConfigVariableDetails(tt.brick, tt.userVariables)
			require.Equal(t, tt.expectedVariableMap, actualVariableMap)
			require.Equal(t, tt.expectedConfigVariables, actualConfigVariables)
		})
	}
}

func TestBricksDetails(t *testing.T) {
	tmpDir := t.TempDir()
	appsDir := filepath.Join(tmpDir, "ArduinoApps")
	dataDir := filepath.Join(tmpDir, "Data")
	assetsDir := filepath.Join(dataDir, "assets")

	require.NoError(t, os.MkdirAll(appsDir, 0755))
	require.NoError(t, os.MkdirAll(assetsDir, 0755))

	brickYaml := filepath.Join(assetsDir, "bricks-list.yaml")
	require.NoError(t, os.WriteFile(brickYaml, []byte(`
bricks:
- id: arduino:object_detection
  name: Object Detection
  description: Detect objects in images using a pre-trained model
  require_container: true
  require_model: true
  mount_devices_into_container: true
  ports: ["8000"]
  category: video
  variables:
  - name: EI_OBJ_DETECTION_MODEL
    description: path to the model file
    default_value: default_path
  - name: CUSTOM_MODEL_PATH
    description: path to the custom model directory
    default_value: /home/arduino/.arduino-bricks/models
- id: arduino:weather_forecast
  name: Weather Forecast
  category:  "miscellaneous"
  model_name: ""
- id: arduino:one_model_brick
  name: one model brick
  category:  "miscellaneous"
  model_name: ""
  `), 0600))

	t.Setenv("ARDUINO_APP_CLI__APPS_DIR", appsDir)
	t.Setenv("ARDUINO_APP_CLI__DATA_DIR", dataDir)

	cfg, err := config.NewFromEnv()
	require.NoError(t, err)

	for _, brick := range []string{"object_detection", "weather_forecast", "one_model_brick"} {
		createFakeBrickAssets(t, assetsDir, brick)
	}
	createFakeApp(t, appsDir)

	bIndex, err := bricksindex.Load(platform.GetPlatform(nil), paths.New(assetsDir))
	require.NoError(t, err)

	mIndex := &modelsindex.ModelsIndex{
		InternalModels: []modelsindex.AIModel{

			{
				ID:                "yolox-object-detection",
				Name:              "General purpose object detection - YoloX",
				ModuleDescription: "General purpose object detection...",
				Bricks:            []modelsindex.BrickConfig{{ID: "arduino:object_detection"}, {ID: "arduino:video_object_detection"}},
			},
			{
				ID:     "face-detection",
				Name:   "Lightweight-Face-Detection",
				Bricks: []modelsindex.BrickConfig{{ID: "arduino:object_detection"}, {ID: "arduino:video_object_detection"}, {ID: "arduino:one_model_brick"}},
			},
		}}

	svc := &Service{
		bricksIndex: bIndex,
		modelsIndex: mIndex,
	}
	idProvider := app.NewAppIDProvider(cfg)

	t.Run("Brick Not Found", func(t *testing.T) {
		res, err := svc.BricksDetails("arduino:non_existing", idProvider, cfg)
		require.Error(t, err)
		require.Equal(t, ErrBrickNotFound, err)
		require.Empty(t, res.ID)
	})

	t.Run("Success - Full Details - multiple models", func(t *testing.T) {
		expectConfigVariables := []BrickConfigVariable{
			{
				Name:        "EI_OBJ_DETECTION_MODEL",
				Value:       "default_path",
				Description: "path to the model file",
				Required:    false,
			},
			{
				Name:        "CUSTOM_MODEL_PATH",
				Value:       "/home/arduino/.arduino-bricks/models",
				Description: "path to the custom model directory",
				Required:    false,
			},
		}

		res, err := svc.BricksDetails("arduino:object_detection", idProvider, cfg)
		require.NoError(t, err)

		require.Equal(t, "arduino:object_detection", res.ID)
		require.Equal(t, "Object Detection", res.Name)
		require.Equal(t, "Arduino", res.Author)
		require.Equal(t, "installed", res.Status)
		require.Contains(t, res.Variables, "EI_OBJ_DETECTION_MODEL")
		require.Equal(t, "default_path", res.Variables["EI_OBJ_DETECTION_MODEL"].DefaultValue)
		require.Equal(t, "# Documentation", res.Readme)
		require.Contains(t, res.ApiDocsPath, filepath.Join("arduino", "app_bricks", "object_detection", "API.md"))
		require.Len(t, res.CodeExamples, 1)
		require.Contains(t, res.CodeExamples[0].Path, "blink.ino")
		require.Len(t, res.UsedByApps, 1)
		require.Equal(t, "My App", res.UsedByApps[0].Name)
		require.NotEmpty(t, res.UsedByApps[0].ID)
		require.Len(t, res.CompatibleModels, 2)
		require.Equal(t, "yolox-object-detection", res.CompatibleModels[0].ID)
		require.Equal(t, "General purpose object detection - YoloX", res.CompatibleModels[0].Name)
		require.Equal(t, "General purpose object detection...", res.CompatibleModels[0].Description)
		require.Equal(t, "face-detection", res.CompatibleModels[1].ID)
		require.Equal(t, "Lightweight-Face-Detection", res.CompatibleModels[1].Name)
		require.Equal(t, "", res.CompatibleModels[1].Description)
		require.Len(t, res.ConfigVariables, 2)
		require.Equal(t, expectConfigVariables, res.ConfigVariables)
	})

	t.Run("Success - Full Details - no models", func(t *testing.T) {
		res, err := svc.BricksDetails("arduino:weather_forecast", idProvider, cfg)
		require.NoError(t, err)

		require.Equal(t, "arduino:weather_forecast", res.ID)
		require.Equal(t, "Weather Forecast", res.Name)
		require.Equal(t, "Arduino", res.Author)
		require.Equal(t, "installed", res.Status)
		require.Empty(t, res.Variables)
		require.Equal(t, "# Documentation", res.Readme)
		require.Contains(t, res.ApiDocsPath, filepath.Join("arduino", "app_bricks", "weather_forecast", "API.md"))
		require.Len(t, res.CodeExamples, 1)
		require.Contains(t, res.CodeExamples[0].Path, "blink.ino")
		require.Len(t, res.UsedByApps, 1)
		require.Equal(t, "My App", res.UsedByApps[0].Name)
		require.NotEmpty(t, res.UsedByApps[0].ID)
		require.Len(t, res.CompatibleModels, 0)
		require.Empty(t, res.ConfigVariables)
	})

	t.Run("Success - Full Details - one model", func(t *testing.T) {
		res, err := svc.BricksDetails("arduino:one_model_brick", idProvider, cfg)
		require.NoError(t, err)

		require.Equal(t, "arduino:one_model_brick", res.ID)
		require.Equal(t, "one model brick", res.Name)
		require.Len(t, res.CompatibleModels, 1)
		require.Equal(t, "face-detection", res.CompatibleModels[0].ID)
		require.Equal(t, "Lightweight-Face-Detection", res.CompatibleModels[0].Name)
		require.Equal(t, "", res.CompatibleModels[0].Description)
		require.Empty(t, res.ConfigVariables)
		require.Empty(t, res.Variables)
	})
}

func createFakeBrickAssets(t *testing.T, assetsDir, brick string) {
	t.Helper()

	readmeDir := filepath.Join(assetsDir, "docs", "arduino", brick)
	require.NoError(t, os.MkdirAll(readmeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(readmeDir, "README.md"),
		[]byte("# Documentation"), 0600))

	brickExDir := filepath.Join(assetsDir, "examples", "arduino", brick)
	require.NoError(t, os.MkdirAll(brickExDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(brickExDir, "blink.ino"),
		[]byte("void setup() {}"), 0600))

	apiDocsDir := filepath.Join(assetsDir, "api-docs", "arduino", "app_bricks", brick)
	require.NoError(t, os.MkdirAll(apiDocsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(apiDocsDir, "API.md"), []byte("# API"), 0600))
}

func createFakeApp(t *testing.T, appsDir string) {
	t.Helper()
	myAppDir := filepath.Join(appsDir, "MyApp")
	require.NoError(t, os.MkdirAll(myAppDir, 0755))

	appYamlContent := `
name: My App
bricks:
  - arduino:object_detection:
  - arduino:weather_forecast:
`
	require.NoError(t, os.WriteFile(filepath.Join(myAppDir, "app.yaml"), []byte(appYamlContent), 0600))
	pythonDir := filepath.Join(myAppDir, "python")
	require.NoError(t, os.MkdirAll(pythonDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pythonDir, "main.py"), []byte("print('hello')"), 0600))
}

func TestAppBrickInstanceModelsDetails(t *testing.T) {

	brickYamlContent := `
bricks:
- id: arduino:object_detection
  name: Object Detection
  require_model: true
  model_name: yolox-object-detection
  category: video
  variables:
  - name: EI_OBJ_DETECTION_MODEL
    description: path to the model file
    default_value: default_path
  - name: CUSTOM_MODEL_PATH
    description: path to the custom model directory
    default_value: /home/arduino/.arduino-bricks/models
- id: arduino:weather_forecast
  name: Weather Forecast
  category:  "miscellaneous"
  model_name: ""
  require_model: false
`
	tmpDir := t.TempDir()
	brickYamlPath := filepath.Join(tmpDir, "bricks-list.yaml")
	require.NoError(t, os.WriteFile(brickYamlPath, []byte(brickYamlContent), 0600))

	bIndex, err := bricksindex.Load(platform.GetPlatform(nil), paths.New(tmpDir))
	require.NoError(t, err)

	mIndex := &modelsindex.ModelsIndex{
		InternalModels: []modelsindex.AIModel{

			{
				ID:                "yolox-object-detection",
				Name:              "General purpose object detection - YoloX",
				ModuleDescription: "General purpose object detection...",
				Bricks:            []modelsindex.BrickConfig{{ID: "arduino:object_detection"}, {ID: "arduino:video_object_detection"}},
			},
			{
				ID:     "face-detection",
				Name:   "Lightweight-Face-Detection",
				Bricks: []modelsindex.BrickConfig{{ID: "arduino:object_detection"}, {ID: "arduino:video_object_detection"}},
			},
		}}

	svc := &Service{
		bricksIndex: bIndex,
		modelsIndex: mIndex,
	}

	tests := []struct {
		name          string
		app           *app.ArduinoApp
		brickID       string
		expectedError string
		validate      func(*testing.T, BrickInstance)
	}{
		{
			name:    "Brick not found in global Index",
			brickID: "arduino:non_existent_brick",
			app: &app.ArduinoApp{
				Descriptor: app.AppDescriptor{Bricks: []app.Brick{}},
			},
			expectedError: "brick not found",
		},
		{
			name:    "Brick found in Index but not added to App",
			brickID: "arduino:object_detection",
			app: &app.ArduinoApp{
				Descriptor: app.AppDescriptor{
					Bricks: []app.Brick{
						{ID: "arduino:weather_forecast"},
					},
				},
			},
			expectedError: "brick arduino:object_detection not added in the app",
		},
		{
			name:    "Success - Standard Brick without Model",
			brickID: "arduino:weather_forecast",
			app: &app.ArduinoApp{
				Descriptor: app.AppDescriptor{
					Bricks: []app.Brick{
						{ID: "arduino:weather_forecast"},
					},
				},
			},
			validate: func(t *testing.T, res BrickInstance) {
				require.Equal(t, "arduino:weather_forecast", res.ID)
				require.Equal(t, "Weather Forecast", res.Name)
				require.Equal(t, "installed", res.Status)
				require.Empty(t, res.ModelID)
				require.Empty(t, res.CompatibleModels)
				require.False(t, res.RequireModel)
			},
		},
		{
			name:    "Success - Brick with Default Model",
			brickID: "arduino:object_detection",
			app: &app.ArduinoApp{
				Descriptor: app.AppDescriptor{
					Bricks: []app.Brick{
						{
							ID: "arduino:object_detection",
						},
					},
				},
			},
			validate: func(t *testing.T, res BrickInstance) {
				require.Equal(t, "arduino:object_detection", res.ID)
				require.Equal(t, "yolox-object-detection", res.ModelID)
				require.Len(t, res.CompatibleModels, 2)
				require.Equal(t, "yolox-object-detection", res.CompatibleModels[0].ID)
				require.Equal(t, "face-detection", res.CompatibleModels[1].ID)
				require.True(t, res.RequireModel)
			},
		},
		{
			name:    "Success - Brick with Overridden Model in App",
			brickID: "arduino:object_detection",
			app: &app.ArduinoApp{
				Descriptor: app.AppDescriptor{
					Bricks: []app.Brick{
						{
							ID:    "arduino:object_detection",
							Model: "face-detection",
						},
					},
				},
			},
			validate: func(t *testing.T, res BrickInstance) {
				require.Equal(t, "arduino:object_detection", res.ID)
				require.Equal(t, "face-detection", res.ModelID)
				require.Len(t, res.CompatibleModels, 2)
				require.Equal(t, "yolox-object-detection", res.CompatibleModels[0].ID)
				require.Equal(t, "face-detection", res.CompatibleModels[1].ID)
				require.True(t, res.RequireModel)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.AppBrickInstanceDetails(tt.app, tt.brickID)

			if tt.expectedError != "" {
				require.Error(t, err)
				require.Equal(t, err.Error(), tt.expectedError)
				return
			}

			require.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestAppBrickInstancesList(t *testing.T) {
	bricksYaml := `bricks:
- id: arduino:weather_forecast
  name: Weather Forecast
  category: miscellaneous
  require_model: false
  variables: []
- id: arduino:object_detection
  name: Object Detection
  category: video
  model_name: yolox-object-detection
  require_model: true
  variables:
  - name: CUSTOM_MODEL_PATH
    default_value: /home/arduino/.arduino-bricks/models
    description: path to the custom model directory
  - name: EI_OBJ_DETECTION_MODEL
    default_value: /models/ootb/ei/yolo-x-nano.eim
    description: path to the model file
- id: arduino:audio_classification
  name: Audio Classification
  category: audio
  model_name: glass-breaking
  require_model: true
  variables:
  - name: CUSTOM_MODEL_PATH
    default_value: /home/arduino/.arduino-bricks/models
  - name: EI_AUDIO_CLASSIFICATION_MODEL
    default_value: /models/ootb/ei/glass-breaking.eim
- id: arduino:streamlit_ui
  name: WebUI - Streamlit
  category: ui
  require_model: false
  ports:
  - "7000"
  - "8000"
- id: arduino:with-hidden-vars
  name: I have some hidden variables
  variables:
  - name: HIDDEN_VAR
    default_value: /i/am/hidden
    hidden: true
  - name: VISIBLE_VAR
    default_value: /i/am/visible
  - name: VISIBLE_VAR_IF_MISSING
    default_value: /i/am/visible
    hidden: false
`

	tmpDir := t.TempDir()
	brickYamlPath := filepath.Join(tmpDir, "bricks-list.yaml")
	require.NoError(t, os.WriteFile(brickYamlPath, []byte(bricksYaml), 0600))

	bIndex, err := bricksindex.Load(platform.GetPlatform(nil), paths.New(tmpDir))
	require.NoError(t, err)

	svc := &Service{
		bricksIndex: bIndex,
		modelsIndex: &modelsindex.ModelsIndex{
			InternalModels: []modelsindex.AIModel{
				{
					ID:                "yolox-object-detection",
					Name:              "General purpose object detection - YoloX",
					ModuleDescription: "a-model-description",
					Bricks:            []modelsindex.BrickConfig{{ID: "arduino:object_detection"}},
				},
				{
					ID:     "face-detection",
					Name:   "Lightweight-Face-Detection",
					Bricks: []modelsindex.BrickConfig{{ID: "arduino:object_detection"}},
				},
			},
		},
	}

	tests := []struct {
		name          string
		app           *app.ArduinoApp
		expectedError string
		validate      func(*testing.T, AppBrickInstancesResult)
	}{
		{
			name: "Brick not found in Index",
			app: &app.ArduinoApp{
				Descriptor: app.AppDescriptor{
					Bricks: []app.Brick{
						{ID: "arduino:non_existent_brick"},
					},
				},
			},
			validate: func(t *testing.T, res AppBrickInstancesResult) {
				require.Len(t, res.BrickInstances, 1)
				brick := res.BrickInstances[0]

				require.Equal(t, "arduino:non_existent_brick", brick.ID)
				require.Equal(t, "arduino:non_existent_brick", brick.Name)
				require.Equal(t, "not_found", brick.Status)
			},
		},
		{
			name: "Success - Empty App",
			app: &app.ArduinoApp{
				Descriptor: app.AppDescriptor{
					Bricks: []app.Brick{},
				},
			},
			validate: func(t *testing.T, res AppBrickInstancesResult) {
				require.Empty(t, res.BrickInstances)
			},
		},
		{
			name: "Success - Simple Brick",
			app: &app.ArduinoApp{
				Descriptor: app.AppDescriptor{
					Bricks: []app.Brick{
						{ID: "arduino:weather_forecast"},
					},
				},
			},
			validate: func(t *testing.T, res AppBrickInstancesResult) {
				require.Len(t, res.BrickInstances, 1)
				brick := res.BrickInstances[0]

				require.Equal(t, "arduino:weather_forecast", brick.ID)
				require.Equal(t, "Weather Forecast", brick.Name)
				require.Equal(t, "miscellaneous", brick.Category)
				require.Equal(t, "installed", brick.Status)
				require.Equal(t, "Arduino", brick.Author)
				require.False(t, brick.RequireModel)
				require.Empty(t, brick.ModelID)
			},
		},
		{
			name: "Success - Brick with Model Configured",
			app: &app.ArduinoApp{
				Descriptor: app.AppDescriptor{
					Bricks: []app.Brick{
						{
							ID:    "arduino:object_detection",
							Model: "face-detection", // default model overridden
							Variables: map[string]string{
								"CUSTOM_MODEL_PATH": "/custom/path",
							},
						},
					},
				},
			},
			validate: func(t *testing.T, res AppBrickInstancesResult) {
				require.Len(t, res.BrickInstances, 1)
				brick := res.BrickInstances[0]

				require.Equal(t, "arduino:object_detection", brick.ID)
				require.Equal(t, "video", brick.Category)
				require.True(t, brick.RequireModel)
				require.Equal(t, "face-detection", brick.ModelID)
				require.Equal(t, []AIModel{
					{ID: "yolox-object-detection", Name: "General purpose object detection - YoloX", Description: "a-model-description"},
					{ID: "face-detection", Name: "Lightweight-Face-Detection", Description: ""},
				}, brick.CompatibleModels)

				foundCustom := false
				for _, v := range brick.ConfigVariables {
					if v.Name == "CUSTOM_MODEL_PATH" {
						require.Equal(t, "/custom/path", v.Value)
						foundCustom = true
					}
				}
				require.True(t, foundCustom, "Variable CUSTOM_MODEL_PATH should be present and overridden")
			},
		},
		{
			name: "Success - Brick using brick default model",
			app: &app.ArduinoApp{
				Descriptor: app.AppDescriptor{
					Bricks: []app.Brick{
						{
							ID: "arduino:object_detection",
						},
					},
				},
			},
			validate: func(t *testing.T, res AppBrickInstancesResult) {
				require.Len(t, res.BrickInstances, 1)
				brick := res.BrickInstances[0]

				require.Equal(t, "arduino:object_detection", brick.ID)
				require.True(t, brick.RequireModel)
				require.Equal(t, "yolox-object-detection", brick.ModelID)
				require.Equal(t, []AIModel{
					{ID: "yolox-object-detection", Name: "General purpose object detection - YoloX", Description: "a-model-description"},
					{ID: "face-detection", Name: "Lightweight-Face-Detection", Description: ""},
				}, brick.CompatibleModels)
			},
		},
		{
			name: "Success - Multiple Bricks",
			app: &app.ArduinoApp{
				Descriptor: app.AppDescriptor{
					Bricks: []app.Brick{
						{ID: "arduino:streamlit_ui"},
						{ID: "arduino:audio_classification", Model: "glass-breaking"},
					},
				},
			},
			validate: func(t *testing.T, res AppBrickInstancesResult) {
				require.Len(t, res.BrickInstances, 2)

				// Brick 1: Streamlit UI
				b1 := res.BrickInstances[0]
				require.Equal(t, "arduino:streamlit_ui", b1.ID)
				require.Equal(t, "WebUI - Streamlit", b1.Name)
				require.Equal(t, "Arduino", b1.Author)
				require.Equal(t, "ui", b1.Category)
				require.Equal(t, "installed", b1.Status)
				require.Equal(t, "", b1.ModelID)
				require.Empty(t, b1.Variables)
				require.Empty(t, b1.ConfigVariables)
				require.False(t, b1.RequireModel)

				// Brick 2: Audio Classification
				b2 := res.BrickInstances[1]
				require.Equal(t, "arduino:audio_classification", b2.ID)
				require.Equal(t, "audio", b2.Category)
				require.True(t, b2.RequireModel)
				require.Equal(t, "glass-breaking", b2.ModelID)
				require.Equal(t, 2, len(b2.ConfigVariables))
				require.Equal(t, "/home/arduino/.arduino-bricks/models", b2.ConfigVariables[0].Value)
				require.Equal(t, "/models/ootb/ei/glass-breaking.eim", b2.ConfigVariables[1].Value)
			},
		},
		{
			name: "Success - hidden variables are not included",
			app: &app.ArduinoApp{
				Descriptor: app.AppDescriptor{
					Bricks: []app.Brick{
						{
							ID: "arduino:with-hidden-vars",
							Variables: map[string]string{
								"HIDDEN_VAR":  "/this/is/a/new/hidden/value",
								"VISIBLE_VAR": "/this/is/a/new/visible/value",
							},
						},
					},
				},
			},
			validate: func(t *testing.T, res AppBrickInstancesResult) {
				require.Len(t, res.BrickInstances, 1)
				brick := res.BrickInstances[0]
				require.Equal(t, "arduino:with-hidden-vars", brick.ID)
				expected := []BrickConfigVariable{
					{Name: "VISIBLE_VAR", Value: "/this/is/a/new/visible/value"},
					{Name: "VISIBLE_VAR_IF_MISSING", Value: "/i/am/visible"},
				}
				require.Equal(t, expected, brick.ConfigVariables)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.AppBrickInstancesList(tt.app)

			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
				return
			}

			require.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestLocalBrickRename(t *testing.T) {
	const sourceApp = "testdata/dummy-app-with-local-brick"
	const tempApp = "testdata/dummy-app-with-local-brick.temp"

	setup := func(t *testing.T) *app.ArduinoApp {
		t.Helper()
		require.NoError(t, paths.New(tempApp).RemoveAll())
		require.NoError(t, paths.New(sourceApp).CopyDirTo(paths.New(tempApp)))
		t.Cleanup(func() { _ = paths.New(tempApp).RemoveAll() })
		a, err := app.Load(paths.New(tempApp))
		require.NoError(t, err)
		return &a
	}

	bricksIndex, err := bricksindex.Load(platform.GetPlatform(nil), paths.New("testdata"))
	require.NoError(t, err)
	svc := NewService(nil, bricksIndex)

	t.Run("fails when old and new id are the same", func(t *testing.T) {
		a := setup(t)
		_, err := svc.LocalBrickRename(a, "my-local-brick", "my-local-brick", "My Local Brick")
		require.Error(t, err)
		require.Contains(t, err.Error(), "same as the current one")
	})

	t.Run("fails when brick is in global index (not local)", func(t *testing.T) {
		a := setup(t)
		_, err := svc.LocalBrickRename(a, "arduino:arduino_cloud", "arduino:arduino_cloud_v2", "Arduino Cloud V2")
		require.ErrorIs(t, err, ErrBrickNotLocal)
	})

	t.Run("fails when brick is not found", func(t *testing.T) {
		a := setup(t)
		_, err := svc.LocalBrickRename(a, "non-existing-brick", "new-id", "New Name")
		require.ErrorIs(t, err, ErrBrickNotFound)
	})

	t.Run("fails when new id conflicts with an existing builtin brick", func(t *testing.T) {
		a := setup(t)
		_, err := svc.LocalBrickRename(a, "my-local-brick", "arduino:arduino_cloud", "Arduino Cloud")
		require.ErrorIs(t, err, ErrBrickIDConflict)
	})

	t.Run("fails when new id conflicts with an existing local brick", func(t *testing.T) {
		a := setup(t)
		_, err := svc.LocalBrickRename(a, "my-local-brick", "another-local-brick", "I want to change the name to another local brick")
		require.ErrorIs(t, err, ErrBrickIDConflict)
	})

	t.Run("successfully renames the local brick", func(t *testing.T) {
		a := setup(t)

		result, err := svc.LocalBrickRename(a, "my-local-brick", "my-renamed-brick", "My Renamed Brick")
		require.NoError(t, err)
		require.Equal(t, "my-renamed-brick", result.ID)

		require.False(t, a.FullPath.Join("bricks", "my-local-brick").Exist())
		require.True(t, a.FullPath.Join("bricks", "my-renamed-brick").Exist())

		configPath := a.FullPath.Join("bricks", "my-renamed-brick", "brick_config.yaml").String()
		raw, err := os.ReadFile(configPath)
		require.NoError(t, err)
		require.Contains(t, string(raw), "my-renamed-brick")
		require.Contains(t, string(raw), "My Renamed Brick")

		appYamlPath := a.FullPath.Join("app.yaml").String()
		appYamlRaw, err := os.ReadFile(appYamlPath)
		require.NoError(t, err)
		require.Contains(t, string(appYamlRaw), "my-renamed-brick")
		require.NotContains(t, string(appYamlRaw), "my-local-brick")
	})

	t.Run("successfully renames a nested local brick", func(t *testing.T) {
		a := setup(t)

		result, err := svc.LocalBrickRename(a, "nested-local-brick", "nested-renamed-brick", "Nested Renamed Brick")
		require.NoError(t, err)
		require.Equal(t, "nested-renamed-brick", result.ID)

		require.False(t, a.FullPath.Join("bricks", "nested", "nested-local-brick").Exist())
		require.True(t, a.FullPath.Join("bricks", "nested", "nested-renamed-brick").Exist())

		configPath := a.FullPath.Join("bricks", "nested", "nested-renamed-brick", "brick_config.yaml").String()
		raw, err := os.ReadFile(configPath)
		require.NoError(t, err)
		require.Contains(t, string(raw), "nested-renamed-brick")
		require.Contains(t, string(raw), "Nested Renamed Brick")

		appYamlPath := a.FullPath.Join("app.yaml").String()
		appYamlRaw, err := os.ReadFile(appYamlPath)
		require.NoError(t, err)
		require.Contains(t, string(appYamlRaw), "nested-renamed-brick")
		require.NotContains(t, string(appYamlRaw), "nested-local-brick")
	})
}
