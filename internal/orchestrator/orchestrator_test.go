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

package orchestrator

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	dockerClient "github.com/docker/docker/client"
	gCmp "github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
)

func TestCloneApp(t *testing.T) {
	cfg := setTestOrchestratorConfig(t)
	idProvider := app.NewAppIDProvider(cfg)

	originalAppID := f.Must(idProvider.ParseID("user:original-app"))
	originalAppPath := originalAppID.ToPath()
	r, err := CreateApp(t.Context(), CreateAppRequest{Name: "original-app"}, idProvider, cfg)
	require.NoError(t, err)
	require.Equal(t, originalAppID, r.ID)
	require.DirExists(t, originalAppPath.String())

	t.Run("valid clone", func(t *testing.T) {
		t.Run("without name", func(t *testing.T) {
			resp, err := CloneApp(t.Context(), CloneAppRequest{FromID: originalAppID}, idProvider, cfg)
			require.NoError(t, err)
			require.Equal(t, f.Must(idProvider.ParseID("user:original-app-copy0")), resp.ID)
			appDir := cfg.AppsDir().Join("original-app-copy0")
			require.DirExists(t, appDir.String())
			t.Cleanup(func() {
				_ = appDir.RemoveAll()
			})

			srcFiles := f.Must(originalAppPath.ReadDir())
			srcFiles.Sort()
			dstFiles := f.Must(appDir.ReadDir())
			dstFiles.Sort()

			require.Len(t, srcFiles, len(dstFiles))

			for i, dstFile := range dstFiles {
				srcFile := srcFiles[i]
				require.Equal(t, srcFile.Base(), dstFile.Base())
				if srcFile.IsDir() {
					require.DirExists(t, dstFile.String())
					require.DirExists(t, srcFile.String())
				} else {
					srcFileContent := f.Must(srcFile.ReadFile())
					dstFileContent := f.Must(dstFile.ReadFile())
					require.Equal(t, dstFileContent, srcFileContent)
				}
			}
		})
		t.Run("with name", func(t *testing.T) {
			resp, err := CloneApp(t.Context(), CloneAppRequest{
				FromID: originalAppID,
				Name:   f.Ptr("new-name"),
			}, idProvider, cfg)
			require.NoError(t, err)
			require.Equal(t, f.Must(idProvider.ParseID("user:new-name")), resp.ID)
			appDir := resp.ID.ToPath()
			require.DirExists(t, appDir.String())
			t.Cleanup(func() {
				_ = appDir.RemoveAll()
			})

			// The app.yaml will have the name set to the new-name
			clonedApp := f.Must(app.Load(appDir))
			require.Equal(t, "new-name", clonedApp.Name)
		})
		t.Run("with icon", func(t *testing.T) {
			resp, err := CloneApp(t.Context(), CloneAppRequest{
				FromID: originalAppID,
				Name:   f.Ptr("with-icon"),
				Icon:   f.Ptr("🦄"),
			}, idProvider, cfg)
			require.NoError(t, err)
			require.Equal(t, f.Must(idProvider.ParseID("user:with-icon")), resp.ID)
			appDir := resp.ID.ToPath()
			require.DirExists(t, appDir.String())
			t.Cleanup(func() {
				_ = appDir.RemoveAll()
			})

			// The app.yaml will have the icon set to 🦄
			clonedApp := f.Must(app.Load(appDir))
			require.Equal(t, "with-icon", clonedApp.Name)
			require.Equal(t, "🦄", clonedApp.Descriptor.Icon)
		})
		t.Run("skips .cache and data folder", func(t *testing.T) {
			baseApp := cfg.AppsDir().Join("app-with-cache")
			require.NoError(t, baseApp.Join(".cache").MkdirAll())
			require.NoError(t, baseApp.Join("data").MkdirAll())
			require.NoError(t, baseApp.Join("app.yaml").WriteFile([]byte("name: app-with-cache")))

			resp, err := CloneApp(t.Context(), CloneAppRequest{FromID: f.Must(idProvider.ParseID("user:app-with-cache"))}, idProvider, cfg)
			require.NoError(t, err)
			require.Equal(t, f.Must(idProvider.ParseID("user:app-with-cache-copy0")), resp.ID)
			appDir := resp.ID.ToPath()
			require.DirExists(t, appDir.String())
			require.NoDirExists(t, appDir.Join(".cache").String())
			require.NoDirExists(t, appDir.Join("data").String())

			t.Cleanup(func() {
				_ = appDir.RemoveAll()
				_ = baseApp.RemoveAll()
			})
		})
	})

	t.Run("invalid app", func(t *testing.T) {
		t.Run("not existing origin", func(t *testing.T) {
			_, err := CloneApp(t.Context(), CloneAppRequest{FromID: f.Must(idProvider.ParseID("user:not-existing"))}, idProvider, cfg)
			require.ErrorIs(t, err, ErrAppDoesntExists)
		})
		t.Run("missing app yaml", func(t *testing.T) {
			err := cfg.AppsDir().Join("app-without-yaml").Mkdir()
			require.NoError(t, err)
			_, err = CloneApp(t.Context(), CloneAppRequest{FromID: f.Must(idProvider.ParseID("user:app-without-yaml"))}, idProvider, cfg)
			require.ErrorIs(t, err, app.ErrInvalidApp)
		})
		t.Run("name already exists", func(t *testing.T) {
			_, err = CloneApp(t.Context(), CloneAppRequest{
				FromID: originalAppID,
				Name:   f.Ptr("original-app"),
			}, idProvider, cfg)
			require.ErrorIs(t, err, ErrAppAlreadyExists)
		})
	})
}

func TestEditApp(t *testing.T) {
	cfg := setTestOrchestratorConfig(t)
	idProvider := app.NewAppIDProvider(cfg)

	t.Run("with default", func(t *testing.T) {
		_, err := CreateApp(t.Context(), CreateAppRequest{Name: "app-default"}, idProvider, cfg)
		require.NoError(t, err)
		appDir := cfg.AppsDir().Join("app-default")

		t.Run("previously not default", func(t *testing.T) {
			app := f.Must(app.Load(appDir))

			previousDefaultApp, err := GetDefaultApp(cfg)
			require.NoError(t, err)
			require.Nil(t, previousDefaultApp)

			err = EditApp(AppEditRequest{Default: f.Ptr(true)}, &app, cfg)
			require.NoError(t, err)

			currentDefaultApp, err := GetDefaultApp(cfg)
			require.NoError(t, err)
			require.True(t, appDir.EquivalentTo(currentDefaultApp.FullPath))
		})
		t.Run("previously default", func(t *testing.T) {
			app := f.Must(app.Load(appDir))
			err := SetDefaultApp(&app, cfg)
			require.NoError(t, err)

			previousDefaultApp, err := GetDefaultApp(cfg)
			require.NoError(t, err)
			require.True(t, appDir.EquivalentTo(previousDefaultApp.FullPath))

			err = EditApp(AppEditRequest{Default: f.Ptr(false)}, &app, cfg)
			require.NoError(t, err)

			currentDefaultApp, err := GetDefaultApp(cfg)
			require.NoError(t, err)
			require.Nil(t, currentDefaultApp)
		})
	})

	t.Run("with name", func(t *testing.T) {
		originalAppName := "original-name"
		_, err := CreateApp(t.Context(), CreateAppRequest{Name: originalAppName}, idProvider, cfg)
		require.NoError(t, err)
		appDir := cfg.AppsDir().Join(originalAppName)
		userApp := f.Must(app.Load(appDir))
		originalPath := userApp.FullPath

		err = EditApp(AppEditRequest{Name: f.Ptr("new-name")}, &userApp, cfg)
		require.NoError(t, err)
		editedApp, err := app.Load(cfg.AppsDir().Join("new-name"))
		require.NoError(t, err)
		require.Equal(t, "new-name", editedApp.Name)
		require.True(t, originalPath.NotExist()) // The original app directory should be removed after renaming

		t.Run("already existing name", func(t *testing.T) {
			existingAppName := "existing-name"
			_, err := CreateApp(t.Context(), CreateAppRequest{Name: existingAppName}, idProvider, cfg)
			require.NoError(t, err)
			appDir := cfg.AppsDir().Join(existingAppName)
			existingApp := f.Must(app.Load(appDir))

			err = EditApp(AppEditRequest{Name: f.Ptr(existingAppName)}, &existingApp, cfg)
			require.ErrorIs(t, err, ErrAppAlreadyExists)
		})
	})

	t.Run("with icon and description", func(t *testing.T) {
		commonAppName := "common-app"
		_, err := CreateApp(t.Context(), CreateAppRequest{Name: commonAppName}, idProvider, cfg)
		require.NoError(t, err)
		commonAppDir := cfg.AppsDir().Join(commonAppName)
		commonApp := f.Must(app.Load(commonAppDir))

		err = EditApp(AppEditRequest{
			Icon:        f.Ptr("💻"),
			Description: f.Ptr("new desc"),
		}, &commonApp, cfg)
		require.NoError(t, err)
		editedApp := f.Must(app.Load(commonAppDir))
		require.Equal(t, "new desc", editedApp.Descriptor.Description)
		require.Equal(t, "💻", editedApp.Descriptor.Icon)
	})
}

func TestListApp(t *testing.T) {
	cfg := setTestOrchestratorConfig(t)
	idProvider := app.NewAppIDProvider(cfg)

	docker, err := dockerClient.NewClientWithOpts(
		dockerClient.FromEnv,
		dockerClient.WithAPIVersionNegotiation(),
	)
	require.NoError(t, err)
	dockerCli, err := command.NewDockerCli(
		command.WithAPIClient(docker),
		command.WithBaseContext(t.Context()),
	)
	require.NoError(t, err)

	err = dockerCli.Initialize(&flags.ClientOptions{})
	require.NoError(t, err)

	createApp(t, "app1", false, idProvider, cfg)
	createApp(t, "app2", false, idProvider, cfg)
	createApp(t, "example1", true, idProvider, cfg)

	t.Run("list all apps", func(t *testing.T) {
		res, err := ListApps(t.Context(), dockerCli, ListAppRequest{
			ShowApps:     true,
			ShowExamples: true,
			StatusFilter: "",
		}, idProvider, cfg)
		require.NoError(t, err)
		assert.Empty(t, res.BrokenApps)
		assert.Empty(t, gCmp.Diff([]AppInfo{
			{
				ID:          f.Must(idProvider.ParseID("examples:example1")),
				Name:        "example1",
				Description: "",
				Icon:        "😃",
				Status:      "uninitialized",
				Example:     true,
				Default:     false,
			},
			{
				ID:          f.Must(idProvider.ParseID("user:app1")),
				Name:        "app1",
				Description: "",
				Icon:        "😃",
				Status:      "uninitialized",
				Example:     false,
				Default:     false,
			},
			{
				ID:          f.Must(idProvider.ParseID("user:app2")),
				Name:        "app2",
				Description: "",
				Icon:        "😃",
				Status:      "uninitialized",
				Example:     false,
				Default:     false,
			},
		}, res.Apps))
	})

	t.Run("list only apps", func(t *testing.T) {
		res, err := ListApps(t.Context(), dockerCli, ListAppRequest{
			ShowApps:     true,
			ShowExamples: false,
			StatusFilter: "",
		}, idProvider, cfg)
		require.NoError(t, err)
		assert.Empty(t, res.BrokenApps)
		assert.Empty(t, gCmp.Diff([]AppInfo{
			{
				ID:          f.Must(idProvider.ParseID("user:app1")),
				Name:        "app1",
				Description: "",
				Icon:        "😃",
				Status:      "uninitialized",
				Example:     false,
				Default:     false,
			},
			{
				ID:          f.Must(idProvider.ParseID("user:app2")),
				Name:        "app2",
				Description: "",
				Icon:        "😃",
				Status:      "uninitialized",
				Example:     false,
				Default:     false,
			},
		}, res.Apps))
	})

	t.Run("list only examples", func(t *testing.T) {
		res, err := ListApps(t.Context(), dockerCli, ListAppRequest{
			ShowApps:     false,
			ShowExamples: true,
			StatusFilter: "",
		}, idProvider, cfg)
		require.NoError(t, err)
		assert.Empty(t, res.BrokenApps)
		assert.Empty(t, gCmp.Diff([]AppInfo{
			{
				ID:          f.Must(idProvider.ParseID("examples:example1")),
				Name:        "example1",
				Description: "",
				Icon:        "😃",
				Status:      "uninitialized",
				Example:     true,
				Default:     false,
			},
		}, res.Apps))
	})
}

func setTestOrchestratorConfig(t *testing.T) config.Configuration {
	t.Helper()

	tmpDir := paths.New(t.TempDir())
	t.Setenv("ARDUINO_APP_CLI__APPS_DIR", tmpDir.Join("apps").String())
	t.Setenv("ARDUINO_APP_CLI__CONFIG_DIR", tmpDir.Join("config").String())
	t.Setenv("ARDUINO_APP_CLI__DATA_DIR", tmpDir.Join("data").String())
	cfg, err := config.NewFromEnv()
	require.NoError(t, err)

	return cfg
}

func createApp(
	t *testing.T,
	name string,
	isExample bool,
	idProvider *app.IDProvider,
	cfg config.Configuration,
) app.ID {
	t.Helper()

	res, err := CreateApp(t.Context(), CreateAppRequest{
		Name: name,
		Icon: "😃",
	}, idProvider, cfg)
	require.NoError(t, err)
	require.Empty(t, gCmp.Diff(f.Must(idProvider.ParseID("user:"+name)), res.ID))
	if isExample {
		newPath := cfg.ExamplesDir().Join(name)
		err = os.Rename(res.ID.ToPath().String(), newPath.String())
		require.NoError(t, err)
		newID, err := idProvider.IDFromPath(newPath)
		require.NoError(t, err)
		assert.Empty(t, gCmp.Diff(f.Must(idProvider.ParseID("examples:"+name)), newID))
		res.ID = newID
	}

	return res.ID
}

func TestSortV4LVideoDevices(t *testing.T) {

	devices := []string{
		"usb-Generic_GENERAL_-_UVC-video-index1",
		"usb-Generic_GENERAL_-_UVC-video-index0",
		"usb-046d_0825-video-index2",
	}

	sortV4lByIndexDevices(devices)
	assert.Equal(t, "usb-Generic_GENERAL_-_UVC-video-index0", devices[0])
	assert.Equal(t, "usb-Generic_GENERAL_-_UVC-video-index1", devices[1])
	assert.Equal(t, "usb-046d_0825-video-index2", devices[2])
}

func TestGetAppEnvironmentVariablesWithDefaults(t *testing.T) {
	cfg := setTestOrchestratorConfig(t)
	idProvider := app.NewAppIDProvider(cfg)

	docker, err := dockerClient.NewClientWithOpts(
		dockerClient.FromEnv,
		dockerClient.WithAPIVersionNegotiation(),
	)
	require.NoError(t, err)
	dockerCli, err := command.NewDockerCli(
		command.WithAPIClient(docker),
		command.WithBaseContext(t.Context()),
	)
	require.NoError(t, err)

	err = dockerCli.Initialize(&flags.ClientOptions{})
	require.NoError(t, err)

	appId := createApp(t, "app1", false, idProvider, cfg)
	appDesc, err := app.Load(appId.ToPath())
	require.NoError(t, err)
	appDesc.Descriptor.Bricks = []app.Brick{
		{
			ID:        "arduino:object_detection",
			Model:     "",                  // use the default model
			Variables: map[string]string{}, // use the default variables
		},
	}

	bricksIndexContent := []byte(`
bricks:
- id: arduino:object_detection
  name: Object Detection
  description: "Brick for object detection using a pre-trained model. It processes\
    \ images and returns the predicted class label, bounding-boxes and confidence\
    \ score.\nBrick is designed to work with pre-trained models provided by framework\
    \ or with custom object detection models trained on Edge Impulse platform. \n"
  require_container: true
  require_model: true
  ports: []
  category: video
  model_name: yolox-object-detection
  variables:
  - name: CUSTOM_MODEL_PATH
    default_value: /home/arduino/.arduino-bricks/ei-models
    description: path to the custom model directory
  - name: EI_OBJ_DETECTION_MODEL
    default_value: /models/ootb/ei/yolo-x-nano.eim
    description: path to the model file
`)
	err = cfg.AssetsDir().Join("bricks-list.yaml").WriteFile(bricksIndexContent)
	require.NoError(t, err)
	bricksIndex, err := bricksindex.Load(cfg.AssetsDir())
	assert.NoError(t, err)

	modelsIndexContent := []byte(`
models:
- yolox-object-detection:
    runner: brick
    name : "General purpose object detection - YoloX"
    description: "General purpose object detection model based on YoloX Nano. This model is trained on the COCO dataset and can detect 80 different object classes."
    model_configuration:
      "EI_OBJ_DETECTION_MODEL": "/models/ootb/ei/yolo-x-nano.eim"
    metadata:
    source: "edgeimpulse"
    ei-project-id: 717280
    source-model-id: "YOLOX-Nano"
    source-model-url: "https://github.com/Megvii-BaseDetection/YOLOX"
    bricks:
    - arduino:object_detection
    - arduino:video_object_detection
`)
	err = cfg.AssetsDir().Join("models-list.yaml").WriteFile(modelsIndexContent)
	require.NoError(t, err)
	modelIndex, err := modelsindex.Load(cfg.AssetsDir())
	require.NoError(t, err)

	env := getAppEnvironmentVariables(appDesc, bricksIndex, modelIndex)
	require.Equal(t, cfg.AppsDir().Join("app1").String(), env["APP_HOME"])
	require.Equal(t, "/models/ootb/ei/yolo-x-nano.eim", env["EI_OBJ_DETECTION_MODEL"])
	require.Equal(t, "/home/arduino/.arduino-bricks/ei-models", env["CUSTOM_MODEL_PATH"])
	// we ignore HOST_IP since it's dynamic
}

func TestGetAppEnvironmentVariablesWithCustomModelOverrides(t *testing.T) {
	cfg := setTestOrchestratorConfig(t)
	idProvider := app.NewAppIDProvider(cfg)

	docker, err := dockerClient.NewClientWithOpts(
		dockerClient.FromEnv,
		dockerClient.WithAPIVersionNegotiation(),
	)
	require.NoError(t, err)
	dockerCli, err := command.NewDockerCli(
		command.WithAPIClient(docker),
		command.WithBaseContext(t.Context()),
	)
	require.NoError(t, err)

	err = dockerCli.Initialize(&flags.ClientOptions{})
	require.NoError(t, err)

	appId := createApp(t, "app1", false, idProvider, cfg)
	appDesc, err := app.Load(appId.ToPath())
	require.NoError(t, err)
	appDesc.Descriptor.Bricks = []app.Brick{
		{
			ID: "arduino:object_detection",
			Variables: map[string]string{
				"EI_OBJ_DETECTION_MODEL": "/home/arduino/.arduino-bricks/ei-models/face-det.eim",
			}, // override the default model via ENV variable
		},
	}

	bricksIndexContent := []byte(`
bricks:
- id: arduino:object_detection
  name: Object Detection
  description: "Brick for object detection using a pre-trained model. It processes\
    \ images and returns the predicted class label, bounding-boxes and confidence\
    \ score.\nBrick is designed to work with pre-trained models provided by framework\
    \ or with custom object detection models trained on Edge Impulse platform. \n"
  require_container: true
  require_model: true
  category: video
  model_name: yolox-object-detection
  variables:
  - name: CUSTOM_MODEL_PATH
    default_value: /home/arduino/.arduino-bricks/ei-models
    description: path to the custom model directory
  - name: EI_OBJ_DETECTION_MODEL
    default_value: /models/ootb/ei/yolo-x-nano.eim
    description: path to the model file
`)
	err = cfg.AssetsDir().Join("bricks-list.yaml").WriteFile(bricksIndexContent)
	require.NoError(t, err)
	bricksIndex, err := bricksindex.Load(cfg.AssetsDir())
	assert.NoError(t, err)

	modelsIndexContent := []byte(`
models:
- yolox-object-detection:
    runner: brick
    name : "General purpose object detection - YoloX"
    description: "General purpose object detection model based on YoloX Nano. This model is trained on the COCO dataset and can detect 80 different object classes."
    model_configuration:
      "EI_OBJ_DETECTION_MODEL": "/models/ootb/ei/yolo-x-nano.eim"
    metadata:
    source: "edgeimpulse"
    ei-project-id: 717280
    source-model-id: "YOLOX-Nano"
    source-model-url: "https://github.com/Megvii-BaseDetection/YOLOX"
    bricks:
    - arduino:object_detection
    - arduino:video_object_detection
`)
	err = cfg.AssetsDir().Join("models-list.yaml").WriteFile(modelsIndexContent)
	require.NoError(t, err)
	modelIndex, err := modelsindex.Load(cfg.AssetsDir())
	require.NoError(t, err)

	env := getAppEnvironmentVariables(appDesc, bricksIndex, modelIndex)
	require.Equal(t, cfg.AppsDir().Join("app1").String(), env["APP_HOME"])
	require.Equal(t, "/home/arduino/.arduino-bricks/ei-models/face-det.eim", env["EI_OBJ_DETECTION_MODEL"])
	require.Equal(t, "/home/arduino/.arduino-bricks/ei-models", env["CUSTOM_MODEL_PATH"])
	// we ignore HOST_IP since it's dynamic
}

func TestValidateDevice(t *testing.T) {

	t.Run("valid", func(t *testing.T) {
		dev := deviceResult{
			devicePaths:    []string{"/dev/video0", "/dev/video1", "/dev/snd/pcmC0D0p"},
			hasGPUDevice:   true,
			hasSoundDevice: true,
			hasVideoDevice: true,
		}
		requiredDeviceClasses := make(map[string]any)
		requiredDeviceClasses["camera"] = true
		requiredDeviceClasses["microphone"] = true
		err := validateDevices(&dev, requiredDeviceClasses)
		assert.NoError(t, err)
	})
	t.Run("no camera", func(t *testing.T) {
		dev := deviceResult{
			devicePaths:    []string{},
			hasGPUDevice:   true,
			hasSoundDevice: false,
			hasVideoDevice: false,
		}
		requiredDeviceClasses := make(map[string]any)
		requiredDeviceClasses["camera"] = true
		err := validateDevices(&dev, requiredDeviceClasses)
		assert.Error(t, err)
	})
	t.Run("no mic", func(t *testing.T) {
		dev := deviceResult{
			devicePaths:    []string{},
			hasGPUDevice:   true,
			hasSoundDevice: false,
			hasVideoDevice: true,
		}
		requiredDeviceClasses := make(map[string]any)
		requiredDeviceClasses["microphone"] = true
		err := validateDevices(&dev, requiredDeviceClasses)
		assert.Error(t, err)
	})
	t.Run("no speaker", func(t *testing.T) {
		dev := deviceResult{
			devicePaths:    []string{},
			hasGPUDevice:   true,
			hasSoundDevice: false,
			hasVideoDevice: true,
		}
		requiredDeviceClasses := make(map[string]any)
		requiredDeviceClasses["speaker"] = true
		err := validateDevices(&dev, requiredDeviceClasses)
		assert.Error(t, err)
	})
}

func TestValidateZipContent(t *testing.T) {
	tests := []struct {
		name          string
		files         map[string]string
		wantErr       bool
		errorContains string
	}{
		{
			name: "Success - Minimal (app.yaml + python)",
			files: map[string]string{
				"app.yaml":       "",
				"python/main.py": "print('hello')",
			},
			wantErr: false,
		},
		{
			name: "Success - Full with Sketch",
			files: map[string]string{
				"app.yaml":           "",
				"python/main.py":     "",
				"sketch/sketch.ino":  "",
				"sketch/sketch.yaml": "",
			},
			wantErr: false,
		},
		{
			name: "Error - Missing app.yaml",
			files: map[string]string{
				"python/main.py": "",
			},
			wantErr:       true,
			errorContains: "missing app.yaml",
		},
		{
			name: "Error - Missing python/main.py",
			files: map[string]string{
				"app.yaml": "",
			},
			wantErr:       true,
			errorContains: "missing python/main.py",
		},
		{
			name: "Error - Sketch folder present but missing .ino",
			files: map[string]string{
				"app.yaml":           "",
				"python/main.py":     "",
				"sketch/sketch.yaml": "",
			},
			wantErr:       true,
			errorContains: "missing .ino file",
		},
		{
			name: "Error - Sketch folder present but missing .yaml",
			files: map[string]string{
				"app.yaml":          "",
				"python/main.py":    "",
				"sketch/sketch.ino": "",
			},
			wantErr:       true,
			errorContains: "missing .yaml file",
		},
		{
			name: "Success - Extra files are allowed",
			files: map[string]string{
				"app.yaml":       "",
				"python/main.py": "",
				"README.md":      "",
				"data/image.png": "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := createMockZip(t, tt.files)

			err := validateZipContent(r)

			if (err != nil) != tt.wantErr {
				t.Errorf("validateZipContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errorContains != "" {
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("validateZipContent() error = %v, expected to contain %v", err, tt.errorContains)
				}
			}
		})
	}
}

func createMockZip(t *testing.T, files map[string]string) *zip.Reader {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.Write([]byte(content))
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestExportAppZip(t *testing.T) {
	type testCase struct {
		name             string
		appName          string
		files            map[string]string
		nonExistent      bool
		includeData      bool
		wantFiles        []string
		wantMissingFiles []string
		wantErr          bool
		wantFilename     string
	}

	tests := []testCase{
		{
			name:    "Standard app name (include_data=false)",
			appName: "My Test App",
			files: map[string]string{
				"app.yaml":     "content",
				"data/foo.txt": "data content",
			},
			includeData:      false,
			wantErr:          false,
			wantFilename:     "my-test-app.zip",
			wantFiles:        []string{"app.yaml"},
			wantMissingFiles: []string{"data/foo.txt"},
		},
		{
			name:    "Include Data directory (include_data=true)",
			appName: "Data App",
			files: map[string]string{
				"app.yaml":     "content",
				"data/foo.txt": "data content",
			},
			includeData:      true,
			wantErr:          false,
			wantFilename:     "data-app.zip",
			wantFiles:        []string{"app.yaml", "data/foo.txt"},
			wantMissingFiles: []string{},
		},
		{
			name:    "Empty app name uses default",
			appName: "",
			files: map[string]string{
				"app.yaml": "content",
			},
			includeData:  false,
			wantErr:      false,
			wantFilename: "app-export.zip",
			wantFiles:    []string{"app.yaml"},
		},
		{
			name:        "Error on non existent path",
			appName:     "Broken App",
			nonExistent: true,
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			for path, content := range tc.files {
				fullPath := filepath.Join(tmpDir, path)
				require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
				require.NoError(t, os.WriteFile(fullPath, []byte(content), 0600))
			}

			appPath := tmpDir
			if tc.nonExistent {
				appPath = filepath.Join(tmpDir, "not-existing")
			}

			app := app.ArduinoApp{
				Name:     tc.appName,
				FullPath: paths.New(appPath),
			}
			zipData, filename, err := ExportAppZip(context.Background(), app, tc.includeData)

			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, zipData)
				require.Empty(t, filename)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.wantFilename, filename)
			require.NotEmpty(t, zipData)

			zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
			require.NoError(t, err)

			presentFiles := make(map[string]bool)
			for _, f := range zipReader.File {
				presentFiles[f.Name] = true
			}

			for _, file := range tc.wantFiles {
				require.True(t, presentFiles[file], "File expected in zip but missing: %s", file)
			}

			for _, file := range tc.wantMissingFiles {
				require.False(t, presentFiles[file], "File should NOT be in zip but was found: %s", file)
			}
		})
	}
}

func TestZipAppToBuffer(t *testing.T) {
	type testCase struct {
		name        string
		files       map[string]string
		nonExistent bool
		includeData bool
		wantErr     bool
		wantInZip   []string
		wantMissing []string
	}

	tests := []testCase{
		{
			name: "Standard happy path",
			files: map[string]string{
				"app.yaml":        "content file",
				"assets/icon.png": "image-data",
			},
			includeData: false,
			wantErr:     false,
			wantInZip:   []string{"app.yaml", "assets/icon.png"},
			wantMissing: []string{},
		},
		{
			name: "Exclude 'data' directory (includeData=false)",
			files: map[string]string{
				"app.yaml":       "content",
				"data/file.txt":  "should be ignored",
				"data/image.png": "should be ignored",
			},
			includeData: false,
			wantErr:     false,
			wantInZip:   []string{"app.yaml"},
			wantMissing: []string{"data/file.txt", "data/image.png"},
		},
		{
			name: "Include 'data' directory (includeData=true)",
			files: map[string]string{
				"app.yaml":      "content",
				"data/file.txt": "should be included",
			},
			includeData: true,
			wantErr:     false,
			wantInZip:   []string{"app.yaml", "data/file.txt"},
			wantMissing: []string{},
		},
		{
			name: "Ignore .cache folder at root",
			files: map[string]string{
				"app.yaml":          "content",
				".cache/temp_file":  "junk",
				".cache/sub/folder": "junk",
			},
			includeData: false,
			wantErr:     false,
			wantInZip:   []string{"app.yaml"},
			wantMissing: []string{".cache/temp_file", ".cache/sub/folder"},
		},
		{
			name: "Include hidden files not in .cache",
			files: map[string]string{
				".env":           "SECRET=123",
				"assets/.hidden": "hidden-asset",
			},
			includeData: false,
			wantErr:     false,
			wantInZip:   []string{".env", "assets/.hidden"},
			wantMissing: []string{},
		},
		{
			name: "Ignore nested directories inside .cache",
			files: map[string]string{
				"app.js":              "code",
				".cache/v1/data.json": "cache-data",
			},
			includeData: false,
			wantErr:     false,
			wantInZip:   []string{"app.js"},
			wantMissing: []string{".cache/v1/data.json"},
		},
		{
			name:        "Error on non-existent path",
			files:       map[string]string{},
			nonExistent: true,
			wantErr:     true,
			wantInZip:   nil,
			wantMissing: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			for path, content := range tc.files {
				fullPath := filepath.Join(tmpDir, path)
				err := os.MkdirAll(filepath.Dir(fullPath), 0755)
				require.NoError(t, err)
				err = os.WriteFile(fullPath, []byte(content), 0600)
				require.NoError(t, err)
			}

			sourcePath := tmpDir
			if tc.nonExistent {
				sourcePath = filepath.Join(tmpDir, "not existing path")
			}
			zipData, err := zipAppToBuffer(sourcePath, tc.includeData)

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, zipData)

			zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
			require.NoError(t, err)

			foundFiles := make(map[string]bool)
			for _, f := range zipReader.File {
				require.False(t, strings.Contains(f.Name, "\\"), "not valid Path separator in %s", f.Name)
				if !f.FileInfo().IsDir() {
					foundFiles[f.Name] = true
				}
			}

			for _, file := range tc.wantInZip {
				require.True(t, foundFiles[file], "Missing file into the zip: %s", file)
			}

			for _, file := range tc.wantMissing {
				require.False(t, foundFiles[file], "present file that should be ignored: %s", file)
			}
		})
	}
}

func TestImportAppFromZip(t *testing.T) {
	type testCase struct {
		name          string
		folderName    string
		zipFiles      map[string]string
		preExisting   bool
		wantErr       bool
		expectedErr   error
		errorContains string
	}

	tests := []testCase{
		{
			name:       "Success - Standard App",
			folderName: "test-app",
			zipFiles: map[string]string{
				"app.yaml":       "name: Test App",
				"python/main.py": "print('hello')",
			},
			wantErr: false,
		},
		{
			name:       "Success - App with Sketch",
			folderName: "app",
			zipFiles: map[string]string{
				"app.yaml":           "name: app",
				"python/main.py":     "pass",
				"sketch/sketch.ino":  "void setup() {}",
				"sketch/sketch.yaml": "board: unoQ",
			},
			wantErr: false,
		},
		{
			name:       "Success - Ignores junk files",
			folderName: "test",
			zipFiles: map[string]string{
				"app.yaml":       "name: test",
				"python/main.py": "print('hello')",
				"junk/._junk":    "garbage",
			},
			wantErr: false,
		},
		{
			name:       "Error - Empty App Name in YAML",
			folderName: "",
			zipFiles: map[string]string{
				"app.yaml":       "name: \"   \"",
				"python/main.py": "print('h')",
			},
			wantErr:       true,
			expectedErr:   ErrBadRequest,
			errorContains: "app name is missing",
		},
		{
			name:       "Error - App Already Exists",
			folderName: "existing-app",
			zipFiles: map[string]string{
				"app.yaml":       "name: Existing App",
				"python/main.py": "print('hello')",
			},
			preExisting: true,
			wantErr:     true,
			expectedErr: ErrAppAlreadyExists,
		},
		{
			name:       "Error - Missing app.yaml",
			folderName: "no-yaml",
			zipFiles: map[string]string{
				"python/main.py": "print('hello')",
			},
			wantErr:       true,
			expectedErr:   ErrBadRequest,
			errorContains: "missing app.yaml",
		},
		{
			name:       "Error - Missing python/main.py",
			folderName: "test",
			zipFiles: map[string]string{
				"app.yaml": "name: test",
			},
			wantErr:       true,
			expectedErr:   ErrBadRequest,
			errorContains: "missing python/main.py",
		},
		{
			name:       "Error - Sketch missing .ino",
			folderName: "broken-sketch",
			zipFiles: map[string]string{
				"app.yaml":           "name: Broken Sketch",
				"python/main.py":     "",
				"sketch/sketch.yaml": "",
			},
			wantErr:       true,
			expectedErr:   ErrBadRequest,
			errorContains: "missing .ino file",
		},
		{
			name:       "Error - Zip Slip Attack",
			folderName: "hacker-app",
			zipFiles: map[string]string{
				"app.yaml":       "name: hacker",
				"python/main.py": "",
				"../../evil.sh":  "echo pwned",
			},
			wantErr:       true,
			errorContains: "illegal file path",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpRoot := t.TempDir()
			appsDirPath := filepath.Join(tmpRoot, "ArduinoApps")

			t.Setenv("ARDUINO_APP_CLI__APPS_DIR", appsDirPath)
			t.Setenv("ARDUINO_APP_CLI__DATA_DIR", filepath.Join(tmpRoot, "Data"))

			cfg, err := config.NewFromEnv()
			require.NoError(t, err)

			idProvider := app.NewAppIDProvider(cfg)

			if tc.preExisting {
				existsPath := filepath.Join(appsDirPath, tc.folderName)
				require.NoError(t, os.MkdirAll(existsPath, 0755))
			}

			zipPath := filepath.Join(tmpRoot, "import.zip")
			createZipFile(t, zipPath, tc.zipFiles)

			id, err := ImportAppFromZip(cfg, paths.New(zipPath), idProvider)

			if tc.wantErr {
				require.Error(t, err)

				if tc.expectedErr != nil {
					require.Truef(t, errors.Is(err, tc.expectedErr), "want error %v, got %v", tc.expectedErr, err)
				}

				if tc.errorContains != "" {
					require.Contains(t, err.Error(), tc.errorContains)
				}

				require.Empty(t, id)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, id)

				finalPath := cfg.AppsDir().Join(tc.folderName)

				require.True(t, finalPath.Exist(), "App folder should exist at %s", finalPath)
				require.True(t, finalPath.Join("app.yaml").Exist(), "app.yaml missing")
				require.True(t, finalPath.Join("python/main.py").Exist(), "main.py missing")

				files, _ := finalPath.Parent().ReadDir()
				for _, f := range files {
					name := f.Base()
					isTempDir := len(name) > 5 && name[:5] == ".tmp_"
					require.False(t, isTempDir, "Temporary folder not cleaned up: %s", name)
				}
			}
		})
	}
}

func createZipFile(t *testing.T, filename string, files map[string]string) {
	t.Helper()
	f, err := os.Create(filename)
	require.NoError(t, err)
	defer f.Close()

	w := zip.NewWriter(f)

	for name, content := range files {
		f, err := w.Create(name)
		require.NoError(t, err)
		_, err = f.Write([]byte(content))
		require.NoError(t, err)
	}

	require.NoError(t, w.Close())
}
