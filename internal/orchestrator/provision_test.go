package orchestrator

import (
	"os"
	"strings"
	"testing"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/store"

	yaml "github.com/goccy/go-yaml"

	"github.com/stretchr/testify/require"
)

func TestProvisionAppWithOverrides(t *testing.T) {
	cfg := setTestOrchestratorConfig(t)
	tempDirectory := t.TempDir()

	// TODO: hack to skip the preEmbargo check
	cfg.UsedPythonImageTag = "latest"

	staticStore := store.NewStaticStore(cfg.AssetsDir().String())

	// Define a mock app with bricks that require overrides
	app := app.ArduinoApp{
		Name: "TestApp",
		Descriptor: app.AppDescriptor{
			Bricks: []app.Brick{
				{
					ID:    "arduino:video_object_detection",
					Model: "yolox-object-detection",
				},
				{
					ID: "arduino:web_ui",
				},
			},
		},
		FullPath: paths.New(tempDirectory),
	}
	require.NoError(t, app.ProvisioningStateDir().MkdirAll())
	// Add compose files for the bricks - video object detection
	videoObjectDetectionComposePath := cfg.AssetsDir().Join("compose", "arduino", "video_object_detection")
	require.NoError(t, videoObjectDetectionComposePath.MkdirAll())
	composeForVideoObjectDetection := `
version: '3.8'
services:
  ei-video-obj-detection-runner:
    image: arduino/video-object-detection:latest
    ports:
    - "8080:8080"
`
	err := videoObjectDetectionComposePath.Join("brick_compose.yaml").WriteFile([]byte(composeForVideoObjectDetection))
	require.NoError(t, err)

	bricksIndexContent := []byte(`
bricks:
- id: arduino:dbstorage_sqlstore
  name: Database Storage - SQLStore
  description: Simplified database storage layer for Arduino sensor data using SQLite
    local database.
  require_container: false
  require_model: false
  ports: []
  category: storage
- id: arduino:video_object_detection
  name: Object Detection
  description: "Brick for object detection using a pre-trained model."
  require_container: true
  require_model: true
  require_devices: true
  ports: []
  category: video
  model_name: yolox-object-detection
  variables:
  - name: CUSTOM_MODEL_PATH
    default_value: /home/arduino/.arduino-bricks/ei-models
    description: path to the custom model directory
  - name: CUSTOM_MODEL_PATH
    default_value: /models/custom/ei/
    description: path to the custom model directory
  - name: EI_OBJ_DETECTION_MODEL
    default_value: /models/ootb/ei/yolo-x-nano.eim
    description: path to the model file`)
	err = cfg.AssetsDir().Join("bricks-list.yaml").WriteFile(bricksIndexContent)
	require.NoError(t, err)

	// Override brick index with custom test content
	bricksIndex, err := bricksindex.GenerateBricksIndexFromFile(cfg.AssetsDir())
	require.Nil(t, err, "Failed to load bricks index with custom content")

	br, ok := bricksIndex.FindBrickByID("arduino:video_object_detection")
	require.True(t, ok, "Brick arduino:video_object_detection should exist in the index")
	require.NotNil(t, br, "Brick arduino:video_object_detection should not be nil")
	require.Equal(t, "Object Detection", br.Name, "Brick name should match")

	// Run the provision function to generate the main compose file
	env := map[string]string{}
	err = generateMainComposeFile(&app, bricksIndex, "app-bricks:python-apps-base:dev-latest", cfg, env, staticStore)

	// Validate that the main compose file and overrides are created
	require.NoError(t, err, "Failed to generate main compose file")
	composeFilePath := paths.New(tempDirectory).Join(".cache").Join("app-compose.yaml")
	require.True(t, composeFilePath.Exist(), "Main compose file should exist")
	overridesFilePath := paths.New(tempDirectory).Join(".cache").Join("app-compose-overrides.yaml")
	require.True(t, overridesFilePath.Exist(), "Override compose file should exist")

	// Open override file and check for the expected override
	overridesContent, err := overridesFilePath.ReadFile()
	require.Nil(t, err, "Failed to read overrides file")
	type services struct {
		Services map[string]map[string]interface{} `yaml:"services"`
	}
	content := services{}
	err = yaml.Unmarshal(overridesContent, &content)
	require.Nil(t, err, "Failed to unmarshal overrides content")
	require.NotNil(t, content.Services["ei-video-obj-detection-runner"], "Override for ei-video-obj-detection-runner should exist")
	require.NotNil(t, content.Services["ei-video-obj-detection-runner"]["devices"], "Override for ei-video-obj-detection-runner devices should exist")
}

func TestVolumeParser(t *testing.T) {

	t.Run("TestPreProvsionVolumesCustomEnv", func(t *testing.T) {
		tempDirectory := t.TempDir()

		volumesFromStrings := `
services:
  dbstorage-influx:
    image: influxdb:2.7
    ports:
      - "${BIND_ADDRESS:-127.0.0.1}:${BIND_PORT:-8086}:8086"
    volumes:
      - "${CUSTOM_PATH:-.}/data/influx-data:/var/lib/influxdb2"
    environment:
      DOCKER_INFLUXDB_INIT_MODE: setup
`
		volumesFromFile := paths.New(tempDirectory).Join("volumes-from.yaml")
		if err := os.WriteFile(volumesFromFile.String(), []byte(volumesFromStrings), 0600); err != nil {
			t.Fatalf("Failed to write volumes from file: %v", err)
		}

		app := &app.ArduinoApp{
			Name:     "TestApp",
			FullPath: paths.New(tempDirectory),
		}
		env := map[string]string{
			"CUSTOM_PATH": tempDirectory,
		}
		volumes, err := extractVolumesFromComposeFile(volumesFromFile.String())
		require.Nil(t, err, "Failed to extract volumes from compose file")
		provisionComposeVolumes(volumesFromFile.String(), volumes, app, env)
		require.True(t, app.FullPath.Join("data").Join("influx-data").Exist(), "Volume directory should exist")
	})

	t.Run("TestPreProvsionVolumesCustomEnvUsingDefault", func(t *testing.T) {
		tempDirectory := t.TempDir()

		volumesFromStrings := `
services:
  dbstorage-influx:
    image: influxdb:2.7
    ports:
      - "${BIND_ADDRESS:-127.0.0.1}:${BIND_PORT:-8086}:8086"
    volumes:
      - "${CUSTOM_PATH:-@@DEFVALUE@@/customized}/data/influx-data:/var/lib/influxdb2"
    environment:
      DOCKER_INFLUXDB_INIT_MODE: setup
`
		volumesFromStrings = strings.ReplaceAll(volumesFromStrings, "@@DEFVALUE@@", tempDirectory)

		volumesFromFile := paths.New(tempDirectory).Join("volumes-from.yaml")
		if err := os.WriteFile(volumesFromFile.String(), []byte(volumesFromStrings), 0600); err != nil {
			t.Fatalf("Failed to write volumes from file: %v", err)
		}

		app := &app.ArduinoApp{
			Name:     "TestApp",
			FullPath: paths.New(tempDirectory),
		}
		// No env, use macro default value
		env := map[string]string{}
		volumes, err := extractVolumesFromComposeFile(volumesFromFile.String())
		require.Nil(t, err, "Failed to extract volumes from compose file")
		provisionComposeVolumes(volumesFromFile.String(), volumes, app, env)
		require.True(t, app.FullPath.Join("customized").Join("data").Join("influx-data").Exist(), "Volume directory should exist")
	})

	t.Run("TestPreProvsionVolumesAsStructure", func(t *testing.T) {
		tempDirectory := t.TempDir()

		volumesFromStrings := `
services:
  dbstorage-influx:
    image: influxdb:2.7
    ports:
      - "${BIND_ADDRESS:-127.0.0.1}:${BIND_PORT:-8086}:8086"
    volumes:
    - type: bind
      source: ${APP_HOME:-.}/data/influx-data
      target: /data/influx-data
    environment:
      DOCKER_INFLUXDB_INIT_MODE: setup
`
		volumesFromFile := paths.New(tempDirectory).Join("volumes-from.yaml")
		if err := os.WriteFile(volumesFromFile.String(), []byte(volumesFromStrings), 0600); err != nil {
			t.Fatalf("Failed to write volumes from file: %v", err)
		}

		app := &app.ArduinoApp{
			Name:     "TestApp",
			FullPath: paths.New(tempDirectory),
		}
		env := map[string]string{}
		volumes, err := extractVolumesFromComposeFile(volumesFromFile.String())
		require.Nil(t, err, "Failed to extract volumes from compose file")
		provisionComposeVolumes(volumesFromFile.String(), volumes, app, env)
		require.True(t, app.FullPath.Join("data").Join("influx-data").Exist(), "Volume directory should exist")
	})

	t.Run("TestPreProvsionVolumes", func(t *testing.T) {
		tempDirectory := t.TempDir()

		volumesFromStrings := `
services:
  dbstorage-influx:
    image: influxdb:2.7
    ports:
      - "${BIND_ADDRESS:-127.0.0.1}:${BIND_PORT:-8086}:8086"
    volumes:
      - "${APP_HOME:-.}/data/influx-data:/var/lib/influxdb2"
    environment:
      DOCKER_INFLUXDB_INIT_MODE: setup
`
		volumesFromFile := paths.New(tempDirectory).Join("volumes-from.yaml")
		if err := os.WriteFile(volumesFromFile.String(), []byte(volumesFromStrings), 0600); err != nil {
			t.Fatalf("Failed to write volumes from file: %v", err)
		}

		app := &app.ArduinoApp{
			Name:     "TestApp",
			FullPath: paths.New(tempDirectory),
		}
		env := map[string]string{}
		volumes, err := extractVolumesFromComposeFile(volumesFromFile.String())
		require.Nil(t, err, "Failed to extract volumes from compose file")
		provisionComposeVolumes(volumesFromFile.String(), volumes, app, env)
		require.True(t, app.FullPath.Join("data").Join("influx-data").Exist(), "Volume directory should exist")
	})

}
