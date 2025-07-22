package orchestrator

import (
	"testing"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"

	yaml "github.com/goccy/go-yaml"

	"github.com/stretchr/testify/assert"
)

func TestProvisionAppWithOverrides(t *testing.T) {

	tempDirectory := t.TempDir()

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
	// Add compose files for the bricks - video object detection
	videoObjectDetectionComposePath := paths.New(tempDirectory).Join(".cache").Join("compose").Join("arduino").Join("video_object_detection")
	err := videoObjectDetectionComposePath.MkdirAll()
	assert.Nil(t, err, "Failed to create compose directory for video object detection")
	composeForVideoObjectDetection := `
version: '3.8'
services:
  ei-video-obj-detection-runner:
    image: arduino/video-object-detection:latest
    ports:
    - "8080:8080"
`
	err = videoObjectDetectionComposePath.Join("brick_compose.yaml").WriteFile([]byte(composeForVideoObjectDetection))
	assert.Nil(t, err, "Failed to write compose file for video object detection")

	// Override brick index with custom test content
	bricksIndex, err := bricksindex.LoadBricksIndex([]byte(`
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
    description: path to the model file`))
	assert.Nil(t, err, "Failed to load bricks index with custom content")

	br, ok := bricksIndex.FindBrickByID("arduino:video_object_detection")
	assert.True(t, ok, "Brick arduino:video_object_detection should exist in the index")
	assert.NotNil(t, br, "Brick arduino:video_object_detection should not be nil")
	assert.Equal(t, "Object Detection", br.Name, "Brick name should match")

	// Run the provision function to generate the main compose file
	err = generateMainComposeFile(&app, bricksIndex, "arduino:appslab-python-apps-base:dev-latest")

	// Validate that the main compose file and overrides are created
	assert.Nil(t, err, "Failed to generate main compose file")
	composeFilePath := paths.New(tempDirectory).Join(".cache").Join("app-compose.yaml")
	assert.True(t, composeFilePath.Exist(), "Main compose file should exist")
	overridesFilePath := paths.New(tempDirectory).Join(".cache").Join("app-compose-overrides.yaml")
	assert.True(t, overridesFilePath.Exist(), "Override compose file should exist")

	// Open override file and check for the expected override
	overridesContent, err := overridesFilePath.ReadFile()
	assert.Nil(t, err, "Failed to read overrides file")
	type services struct {
		Services map[string]map[string]interface{} `yaml:"services"`
	}
	content := services{}
	err = yaml.Unmarshal(overridesContent, &content)
	assert.Nil(t, err, "Failed to unmarshal overrides content")
	assert.NotNil(t, content.Services["ei-video-obj-detection-runner"], "Override for ei-video-obj-detection-runner should exist")
	assert.NotNil(t, content.Services["ei-video-obj-detection-runner"]["devices"], "Override for ei-video-obj-detection-runner devices should exist")
}
