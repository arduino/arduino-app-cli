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

package orchestrator

import (
	"os"
	"strings"
	"testing"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/peripherals"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/platform"

	"github.com/goccy/go-yaml"

	"github.com/stretchr/testify/require"
)

var unkownPlatform = platform.Platform{}

func TestProvisionAppWithOverrides(t *testing.T) {
	cfg := setTestOrchestratorConfig(t)
	tempDirectory := t.TempDir()

	// Define a mock app with bricks that require overrides
	app := app.ArduinoApp{
		Name: "TestApp",
		Descriptor: app.AppDescriptor{
			Bricks: []app.Brick{
				{
					ID:    "arduino:video_object_detection",
					Model: "yolox-object-detection",
					Variables: map[string]string{
						"CUSTOM_MODEL_PATH": "/models/custom/ei/",
					},
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
  mount_devices_into_container: true
  ports: []
  category: video
  model_name: yolox-object-detection
  variables:
  - name: CUSTOM_MODEL_PATH
    default_value: /home/arduino/.arduino-bricks/models
    description: path to the custom model directory
  - name: CUSTOM_MODEL_PATH
    default_value: /models/custom/ei/
    description: path to the custom model directory
  - name: EI_OBJ_DETECTION_MODEL
    default_value: /models/ootb/ei/yolo-x-nano.eim
    description: path to the model file`)
	err = cfg.AssetsDir().Join("bricks-list.yaml").WriteFile(bricksIndexContent)
	require.NoError(t, err)

	require.NoError(t, cfg.AssetsDir().Join("services").MkdirAll())
	servicesIndex, err := servicesindex.Load(cfg.AssetsDir().Join("services"))
	require.NoError(t, err, "Failed to load services index")

	// Override brick index with custom test content
	bricksIndex, err := bricksindex.Load(cfg.AssetsDir())
	require.Nil(t, err, "Failed to load bricks index with custom content")

	br, ok := bricksIndex.FindBrickByID("arduino:video_object_detection")
	require.True(t, ok, "Brick arduino:video_object_detection should exist in the index")
	require.NotNil(t, br, "Brick arduino:video_object_detection should not be nil")
	require.Equal(t, "Object Detection", br.Name, "Brick name should match")

	// Run the provision function to generate the main compose file
	env := map[string]string{
		"FOO": "bar",
	}

	devices := peripherals.AvailableDevices{
		DevicePaths:    []string{},
		HasGPUDevice:   true,
		HasSoundDevice: false,
		HasVideoDevice: true,
	}
	err = generateMainComposeFile(&app, bricksIndex, servicesIndex, "app-bricks:python-apps-base:dev-latest", cfg, env, unkownPlatform, devices)

	// Validate that the main compose file and overrides are created
	require.NoError(t, err, "Failed to generate main compose file")
	composeFilePath := paths.New(tempDirectory).Join(".cache").Join("app-compose.yaml")
	require.True(t, composeFilePath.Exist(), "Main compose file should exist")
	overridesFilePath := paths.New(tempDirectory).Join(".cache").Join("app-compose-overrides.yaml")
	require.True(t, overridesFilePath.Exist(), "Override compose file should exist")

	// Open override file and check for the expected override
	overridesContent, err := overridesFilePath.ReadFile()
	require.NoError(t, err)

	type services struct {
		Services map[string]map[string]interface{} `yaml:"services"`
	}
	content := services{}
	err = yaml.Unmarshal(overridesContent, &content)
	require.Nil(t, err, "Failed to unmarshal overrides content")
	require.NotNil(t, content.Services["ei-video-obj-detection-runner"], "Override for ei-video-obj-detection-runner should exist")
	require.NotNil(t, content.Services["ei-video-obj-detection-runner"]["devices"], "Override for ei-video-obj-detection-runner devices should exist")
	require.Equal(t, "bar", content.Services["ei-video-obj-detection-runner"]["environment"].(map[string]interface{})["FOO"])
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

	t.Run("TestPreProvsionVolumesWithNestedEnv", func(t *testing.T) {
		tempDirectory := t.TempDir()

		volumesFromStrings := `
services:
  dbstorage-influx:
    image: influxdb:2.7
    ports:
      - "${BIND_ADDRESS:-127.0.0.1}:${BIND_PORT:-8086}:8086"
    volumes:
      - "${CUSTOM_PATH:-${DEFVALUE}/customized}/data/influx-data:/var/lib/influxdb2"
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
		// Use env for nested default value
		os.Setenv("DEFVALUE", tempDirectory)

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

func TestProvisionAppWithDependsOn(t *testing.T) {
	cfg := setTestOrchestratorConfig(t)
	tempDirectory := t.TempDir()
	var env = map[string]string{}
	type services struct {
		Services map[string]struct {
			Image     string `yaml:"image"`
			DependsOn map[string]struct {
				Condition string `yaml:"condition"`
			} `yaml:"depends_on"`
		} `yaml:"services"`
	}

	bricksIndexContent := []byte(`
bricks:
- id: arduino:dbstorage_tsstore
  name: Database Storage - Time Series Store
  description: Simplified time series database storage layer for Arduino sensor samples
    built on top of InfluxDB.
  require_container: true
  require_model: false
  ports: []
  category: storage
  variables:
  - name: APP_HOME
    default_value: .`)
	err := cfg.AssetsDir().Join("bricks-list.yaml").WriteFile(bricksIndexContent)
	require.NoError(t, err)

	require.NoError(t, cfg.AssetsDir().Join("services").MkdirAll())
	servicesIndex, err := servicesindex.Load(cfg.AssetsDir().Join("services"))
	require.NoError(t, err, "Failed to load services index")

	bricksIndex, err := bricksindex.Load(cfg.AssetsDir())
	require.Nil(t, err, "Failed to load bricks index with custom content")
	br, ok := bricksIndex.FindBrickByID("arduino:dbstorage_tsstore")
	require.True(t, ok, "Brick arduino:dbstorage_tsstore should exist in the index")
	require.NotNil(t, br, "Brick arduino:dbstorage_tsstore should not be nil")
	require.Equal(t, "Database Storage - Time Series Store", br.Name, "Brick name should match")

	app := app.ArduinoApp{
		Name: "TestApp",
		Descriptor: app.AppDescriptor{
			Bricks: []app.Brick{
				{
					ID: "arduino:dbstorage_tsstore",
				},
			},
		},
		FullPath: paths.New(tempDirectory),
	}
	require.NoError(t, app.ProvisioningStateDir().MkdirAll())

	t.Run("services with healthcheck", func(t *testing.T) {
		fileComposePath := cfg.AssetsDir().Join("compose", "arduino", "dbstorage_tsstore")
		require.NoError(t, fileComposePath.MkdirAll())
		dependsOnFromStrings := `
services:
  dbstorage-influx:
    image: influxdb:2.7
    ports:
      - "${BIND_ADDRESS:-127.0.0.1}:${BIND_PORT:-8086}:8086"
    volumes:
      - "${APP_HOME:-.}/data/influx-data:/var/lib/influxdb2"
    environment:
      DOCKER_INFLUXDB_INIT_MODE: setup
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8086/health"]`
		err := fileComposePath.Join("brick_compose.yaml").WriteFile([]byte(dependsOnFromStrings))
		require.NoError(t, err)

		devices := peripherals.AvailableDevices{
			DevicePaths:    []string{},
			HasGPUDevice:   true,
			HasSoundDevice: false,
			HasVideoDevice: true,
		}

		// Run the provision function to generate the main compose file
		err = generateMainComposeFile(&app, bricksIndex, servicesIndex, "app-bricks:python-apps-base:dev-latest", cfg, env, unkownPlatform, devices)
		require.NoError(t, err, "Failed to generate main compose file")
		composeFilePath := paths.New(tempDirectory).Join(".cache").Join("app-compose.yaml")
		require.True(t, composeFilePath.Exist(), "Main compose file should exist")

		// Open main compose file and check for the expected depends_on with service_healthy
		mainComposeFileContent, err := composeFilePath.ReadFile()
		require.Nil(t, err, "Failed to read compose file")
		var content services
		err = yaml.Unmarshal(mainComposeFileContent, &content)
		require.Nil(t, err, "Failed to unmarshal overrides content")
		exp := services{
			Services: map[string]struct {
				Image     string `yaml:"image"`
				DependsOn map[string]struct {
					Condition string `yaml:"condition"`
				} `yaml:"depends_on"`
			}{
				"main": {
					Image: "app-bricks:python-apps-base:dev-latest",
					DependsOn: map[string]struct {
						Condition string `yaml:"condition"`
					}{
						"dbstorage-influx": {
							Condition: "service_healthy",
						},
					},
				},
			},
		}
		require.Equal(t, exp, content, "Main compose content should match the expected structure")
	})

	t.Run("services without healthcheck", func(t *testing.T) {
		fileComposePath := cfg.AssetsDir().Join("compose", "arduino", "dbstorage_tsstore")
		require.NoError(t, fileComposePath.MkdirAll())
		dependsOnFromStrings := `
services:
  dbstorage-influx:
    image: influxdb:2.7
    ports:
      - "${BIND_ADDRESS:-127.0.0.1}:${BIND_PORT:-8086}:8086"
    volumes:
      - "${APP_HOME:-.}/data/influx-data:/var/lib/influxdb2"
    environment:
      DOCKER_INFLUXDB_INIT_MODE: setup`
		err = fileComposePath.Join("brick_compose.yaml").WriteFile([]byte(dependsOnFromStrings))
		require.NoError(t, err)

		devices := peripherals.AvailableDevices{
			DevicePaths:    []string{},
			HasGPUDevice:   true,
			HasSoundDevice: false,
			HasVideoDevice: true,
		}
		// Run the provision function to generate the main compose file
		err = generateMainComposeFile(&app, bricksIndex, servicesIndex, "app-bricks:python-apps-base:dev-latest", cfg, env, unkownPlatform, devices)
		require.NoError(t, err, "Failed to generate main compose file")
		composeFilePath := paths.New(tempDirectory).Join(".cache").Join("app-compose.yaml")
		require.True(t, composeFilePath.Exist(), "Main compose file should exist")

		// Open main compose file and check for the expected depends_on with service_started
		mainComposeFileContent, err := composeFilePath.ReadFile()
		require.Nil(t, err, "Failed to read compose file")
		var content services
		err = yaml.Unmarshal(mainComposeFileContent, &content)
		require.Nil(t, err, "Failed to unmarshal overrides content")
		exp := services{
			Services: map[string]struct {
				Image     string `yaml:"image"`
				DependsOn map[string]struct {
					Condition string `yaml:"condition"`
				} `yaml:"depends_on"`
			}{
				"main": {
					Image: "app-bricks:python-apps-base:dev-latest",
					DependsOn: map[string]struct {
						Condition string `yaml:"condition"`
					}{
						"dbstorage-influx": {
							Condition: "service_started",
						},
					},
				},
			},
		}
		require.Equal(t, exp, content, "Main compose content should match the expected structure")
	})
}

func TestProvisionAppComposeOverridesFile(t *testing.T) {
	cfg := setTestOrchestratorConfig(t)
	// staticStore := store.NewStaticStore(cfg.AssetsDir().String())
	tempDirectory := t.TempDir()
	var env = map[string]string{}
	type services struct {
		Services map[string]struct {
			User      *string `yaml:"user"`
			Image     string  `yaml:"image"`
			DependsOn map[string]struct {
				Condition string `yaml:"condition"`
			} `yaml:"depends_on"`
		} `yaml:"services"`
	}

	bricksIndexContent := []byte(`
bricks:
- id: arduino:dbstorage_tsstore
  name: Database Storage - Time Series Store
  description: Simplified time series database storage layer for Arduino sensor samples
    built on top of InfluxDB.
  require_container: true
  require_model: false
  ports: []
  category: storage
  variables:
  - name: APP_HOME
    default_value: .`)
	err := cfg.AssetsDir().Join("bricks-list.yaml").WriteFile(bricksIndexContent)
	require.NoError(t, err)

	require.NoError(t, cfg.AssetsDir().Join("services").MkdirAll())
	servicesIndex, err := servicesindex.Load(cfg.AssetsDir().Join("services"))
	require.NoError(t, err, "Failed to load services index")

	bricksIndex, err := bricksindex.Load(cfg.AssetsDir())
	require.Nil(t, err, "Failed to load bricks index with custom content")
	br, ok := bricksIndex.FindBrickByID("arduino:dbstorage_tsstore")
	require.True(t, ok, "Brick arduino:dbstorage_tsstore should exist in the index")
	require.NotNil(t, br, "Brick arduino:dbstorage_tsstore should not be nil")
	require.Equal(t, "Database Storage - Time Series Store", br.Name, "Brick name should match")

	app := app.ArduinoApp{
		Name: "TestApp",
		Descriptor: app.AppDescriptor{
			Bricks: []app.Brick{
				{
					ID: "arduino:dbstorage_tsstore",
				},
			},
		},
		FullPath: paths.New(tempDirectory),
	}
	require.NoError(t, app.ProvisioningStateDir().MkdirAll())

	t.Run("services with user override", func(t *testing.T) {
		fileComposePath := cfg.AssetsDir().Join("compose", "arduino", "dbstorage_tsstore")
		require.NoError(t, fileComposePath.MkdirAll())
		dependsOnFromStrings := `
services:
  dbstorage-influx:
    image: influxdb:2.7
    user: 0:0
    ports:
      - "${BIND_ADDRESS:-127.0.0.1}:${BIND_PORT:-8086}:8086"
    volumes:
      - "${APP_HOME:-.}/data/influx-data:/var/lib/influxdb2"
    environment:
      DOCKER_INFLUXDB_INIT_MODE: setup
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8086/health"]
  dbstorage-influx-2:
    image: influxdb:2.7
    ports:
      - "${BIND_ADDRESS:-127.0.0.1}:${BIND_PORT:-8086}:8086"
    volumes:
      - "${APP_HOME:-.}/data/influx-data:/var/lib/influxdb2"
    environment:
      DOCKER_INFLUXDB_INIT_MODE: setup
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8086/health"]`
		serviceComposeFilePath := fileComposePath.Join("brick_compose.yaml")
		err := serviceComposeFilePath.WriteFile([]byte(dependsOnFromStrings))
		require.NoError(t, err)

		availableDevices := peripherals.AvailableDevices{
			DevicePaths:    []string{},
			HasGPUDevice:   true,
			HasSoundDevice: false,
			HasVideoDevice: true,
		}
		// Run the provision function to generate the main compose file
		err = generateMainComposeFile(&app, bricksIndex, servicesIndex, "app-bricks:python-apps-base:dev-latest", cfg, env, unkownPlatform, availableDevices)
		require.NoError(t, err, "Failed to generate main compose file")
		composeFilePath := paths.New(tempDirectory).Join(".cache").Join("app-compose.yaml")
		require.True(t, composeFilePath.Exist(), "Main compose file should exist")

		// Extract services from the compose file to prepare override generation
		svcInfo, err := extractServicesFromComposeFile(serviceComposeFilePath)
		require.NoError(t, err)
		devices := []string{"/dev/ttyUSB0:/dev/ttyUSB0"}

		user := "1000:1000"

		groups := []uint32{20} // dialout group ID

		// Generate overrides file
		overrideComposeFile := paths.New(tempDirectory).Join(".cache").Join("app-compose-overrides.yaml")
		err = generateServicesOverrideFile(&app, svcInfo, devices, user, groups, overrideComposeFile, env)
		require.NoError(t, err)

		// load and validate override file content
		overrideComposeFileContent, err := overrideComposeFile.ReadFile()
		require.NoError(t, err)
		var content services
		err = yaml.Unmarshal(overrideComposeFileContent, &content)
		require.NoError(t, err)
		for svcName, svc := range content.Services {
			if svcName != "dbstorage-influx" {
				require.NotNil(t, svc.User, "User override should be present for dbstorage-influx-2")
				require.Equal(t, user, *svc.User, "User override should match the expected value for dbstorage-influx-2")
			} else {
				require.Nil(t, svc.User, "User override should not be present for dbstorage-influx")
			}
		}
	})

}

func TestProvisionAppWithServices(t *testing.T) {
	cfg := setTestOrchestratorConfig(t)
	tempDirectory := t.TempDir()

	// Define a mock app with bricks that require overrides
	app := app.ArduinoApp{
		Name: "TestApp",
		Descriptor: app.AppDescriptor{
			Bricks: []app.Brick{
				{
					ID: "arduino:video_object_detection",
				},
			},
		},
		FullPath: paths.New(tempDirectory),
	}
	require.NoError(t, app.ProvisioningStateDir().MkdirAll())
	// Add compose files for the bricks - video object detection
	videoObjectDetectionPath := cfg.AssetsDir().Join("services", "arduino", "video_object_detection")
	require.NoError(t, videoObjectDetectionPath.MkdirAll())
	composeForVideoObjectDetection := `
version: '3.8'
services:
  ei-video-obj-detection-runner:
    image: arduino/video-object-detection:latest
    ports:
    - "8080:8080"
`
	err := videoObjectDetectionPath.Join("service_compose.yaml").WriteFile([]byte(composeForVideoObjectDetection))
	require.NoError(t, err)

	configForVideoObjectDetection := `
service_id: arduino:foo
name: Foo Service
description: |
  This is a sample Foo service used for testing purposes.
category: test
supported_boards: ["foobar"]
`
	err = videoObjectDetectionPath.Join("service_config.yaml").WriteFile([]byte(configForVideoObjectDetection))
	require.NoError(t, err)

	bricksIndexContent := []byte(`
bricks:
- id: arduino:video_object_detection
  name: Object Detection
  description: "Brick for object detection using a pre-trained model."
  require_container: false
  require_model: false
  ports: []
  category: video
  requires_services: ["arduino:foo"]`)
	err = cfg.AssetsDir().Join("bricks-list.yaml").WriteFile(bricksIndexContent)
	require.NoError(t, err)
	servicesIndex, err := servicesindex.Load(cfg.AssetsDir().Join("services"))
	require.NoError(t, err, "Failed to load services index")

	// Override brick index with custom test content
	bricksIndex, err := bricksindex.Load(cfg.AssetsDir())
	require.Nil(t, err, "Failed to load bricks index with custom content")

	br, ok := bricksIndex.FindBrickByID("arduino:video_object_detection")
	require.True(t, ok, "Brick arduino:video_object_detection should exist in the index")
	require.NotNil(t, br, "Brick arduino:video_object_detection should not be nil")
	require.Equal(t, "Object Detection", br.Name, "Brick name should match")

	service, ok := servicesIndex.FindServiceByID("arduino:foo")
	require.True(t, ok, "Service arduino:foo should exist in the index")
	require.NotNil(t, service, "Service arduino:foo should not be nil")
	compose, ok := service.GetComposeFile()
	require.True(t, ok, "Service arduino:foo should have a compose file")
	require.Equal(t, videoObjectDetectionPath.Join("service_compose.yaml").String(), compose.String())

	// Run the provision function to generate the main compose file
	env := map[string]string{
		"FOO": "bar",
	}

	err = generateMainComposeFile(&app, bricksIndex, servicesIndex, "app-bricks:python-apps-base:dev-latest", cfg, env, unkownPlatform, peripherals.AvailableDevices{})

	// Validate that the main compose file and overrides are created
	require.NoError(t, err, "Failed to generate main compose file")
	composeFilePath := paths.New(tempDirectory).Join(".cache").Join("app-compose.yaml")
	require.True(t, composeFilePath.Exist(), "Main compose file should exist")
	overridesFilePath := paths.New(tempDirectory).Join(".cache").Join("app-compose-overrides.yaml")
	require.True(t, overridesFilePath.Exist(), "Override compose file should exist")

	// Open override file and check for the expected override
	overridesContent, err := overridesFilePath.ReadFile()
	require.NoError(t, err)

	type services struct {
		Services map[string]map[string]interface{} `yaml:"services"`
	}
	content := services{}
	err = yaml.Unmarshal(overridesContent, &content)
	require.Nil(t, err, "Failed to unmarshal overrides content")
	require.NotNil(t, content.Services["ei-video-obj-detection-runner"], "Override for ei-video-obj-detection-runner should exist")
	require.Equal(t, "bar", content.Services["ei-video-obj-detection-runner"]["environment"].(map[string]interface{})["FOO"])
}
