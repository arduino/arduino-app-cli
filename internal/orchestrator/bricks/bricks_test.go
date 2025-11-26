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
	"github.com/arduino/arduino-app-cli/internal/store"
)

func TestBrickCreate(t *testing.T) {
	bricksIndex, err := bricksindex.Load(paths.New("testdata"))
	require.Nil(t, err)
	brickService := NewService(nil, bricksIndex, nil)

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
		bricksIndex, err := bricksindex.Load(paths.New("testdata"))
		require.Nil(t, err)
		brickService := NewService(nil, bricksIndex, nil)

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
	bricksIndex, err := bricksindex.Load(paths.New("testdata"))
	require.Nil(t, err)
	brickService := NewService(nil, bricksIndex, nil)

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
		bricksIndex, err := bricksindex.Load(paths.New("testdata"))
		require.Nil(t, err)
		brickService := NewService(nil, bricksIndex, nil)

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
		bricksIndex, err := bricksindex.Load(paths.New("testdata"))
		require.Nil(t, err)
		brickService := NewService(nil, bricksIndex, nil)

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualVariableMap, actualConfigVariables := getBrickConfigDetails(tt.brick, tt.userVariables)
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

	t.Setenv("ARDUINO_APP_CLI__APPS_DIR", appsDir)
	t.Setenv("ARDUINO_APP_CLI__DATA_DIR", dataDir)

	cfg, err := config.NewFromEnv()
	require.NoError(t, err)

	for _, brick := range []string{"object_detection", "weather_forecast", "one_model_brick"} {
		createFakeBrickAssets(t, assetsDir, brick)
	}
	createFakeApp(t, appsDir)

	bIndex := &bricksindex.BricksIndex{
		Bricks: []bricksindex.Brick{
			{
				ID:        "arduino:object_detection",
				Name:      "Object Detection",
				Category:  "video",
				ModelName: "yolox-object-detection", // Default model
				Variables: []bricksindex.BrickVariable{
					{Name: "EI_OBJ_DETECTION_MODEL", DefaultValue: "default_path", Description: "path to the model file"},
					{Name: "CUSTOM_MODEL_PATH", DefaultValue: "/home/arduino/.arduino-bricks/ei-models", Description: "path to the custom model directory"},
				},
			},
			{
				ID:        "arduino:weather_forecast",
				Name:      "Weather Forecast",
				Category:  "miscellaneous",
				ModelName: "",
			},
			{
				ID:        "arduino:one_model_brick",
				Name:      "one model brick",
				Category:  "video",
				ModelName: "face-detection", // Default model
				Variables: []bricksindex.BrickVariable{},
			},
		},
	}
	mIndex := &modelsindex.ModelsIndex{
		Models: []modelsindex.AIModel{

			{
				ID:                "yolox-object-detection",
				Name:              "General purpose object detection - YoloX",
				ModuleDescription: "General purpose object detection...",
				Bricks:            []string{"arduino:object_detection", "arduino:video_object_detection"},
			},
			{
				ID:     "face-detection",
				Name:   "Lightweight-Face-Detection",
				Bricks: []string{"arduino:object_detection", "arduino:video_object_detection", "arduino:one_model_brick"},
			},
		}}

	svc := &Service{
		bricksIndex: bIndex,
		modelsIndex: mIndex,
		staticStore: store.NewStaticStore(assetsDir),
	}
	idProvider := app.NewAppIDProvider(cfg)

	t.Run("Brick Not Found", func(t *testing.T) {
		res, err := svc.BricksDetails("arduino:non_existing", idProvider, cfg)
		require.Error(t, err)
		require.Equal(t, ErrBrickNotFound, err)
		require.Empty(t, res.ID)
	})

	t.Run("Success - Full Details - multiple models", func(t *testing.T) {
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
	})
}

func createFakeBrickAssets(t *testing.T, assetsDir, brick string) {
	t.Helper()

	brickDocDir := filepath.Join(assetsDir, "docs", "arduino", brick)
	require.NoError(t, os.MkdirAll(brickDocDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(brickDocDir, "README.md"),
		[]byte("# Documentation"), 0600))

	brickExDir := filepath.Join(assetsDir, "examples", "arduino", brick)
	require.NoError(t, os.MkdirAll(brickExDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(brickExDir, "blink.ino"),
		[]byte("void setup() {}"), 0600))
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

	bIndex := &bricksindex.BricksIndex{
		Bricks: []bricksindex.Brick{
			{
				ID:        "arduino:object_detection",
				Name:      "Object Detection",
				Category:  "video",
				ModelName: "yolox-object-detection", // Default model
				Variables: []bricksindex.BrickVariable{
					{Name: "EI_OBJ_DETECTION_MODEL", DefaultValue: "default_path", Description: "path to the model file"},
					{Name: "CUSTOM_MODEL_PATH", DefaultValue: "/home/arduino/.arduino-bricks/ei-models", Description: "path to the custom model directory"},
				},
			},
			{
				ID:        "arduino:weather_forecast",
				Name:      "Weather Forecast",
				Category:  "miscellaneous",
				ModelName: "",
			},
		},
	}

	mIndex := &modelsindex.ModelsIndex{
		Models: []modelsindex.AIModel{

			{
				ID:                "yolox-object-detection",
				Name:              "General purpose object detection - YoloX",
				ModuleDescription: "General purpose object detection...",
				Bricks:            []string{"arduino:object_detection", "arduino:video_object_detection"},
			},
			{
				ID:     "face-detection",
				Name:   "Lightweight-Face-Detection",
				Bricks: []string{"arduino:object_detection", "arduino:video_object_detection"},
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
