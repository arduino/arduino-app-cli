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
	"github.com/arduino/arduino-app-cli/internal/platform"
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
				Name:   new("new-name"),
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
				Name:   new("with-icon"),
				Icon:   new("🦄"),
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
				Name:   new("original-app"),
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

			err = EditApp(AppEditRequest{Default: new(true)}, &app, cfg)
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

			err = EditApp(AppEditRequest{Default: new(false)}, &app, cfg)
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

		err = EditApp(AppEditRequest{Name: new("new-name")}, &userApp, cfg)
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

			err = EditApp(AppEditRequest{Name: new(existingAppName)}, &existingApp, cfg)
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
			Icon:        new("💻"),
			Description: new("new desc"),
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
		}, idProvider, nil, cfg)
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
		}, idProvider, nil, cfg)
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
		}, idProvider, nil, cfg)
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

	t.Run("ignore temporary apps starting with .tmp_", func(t *testing.T) {
		tmpAppPath, err := app.MkTmpAppDir(cfg.AppsDir())
		require.NoError(t, err)

		tmpAppName := tmpAppPath.Base()
		require.True(t, strings.HasPrefix(tmpAppName, ".tmp_"))

		_, err = os.Create(tmpAppPath.Join("app.yaml").String())
		require.NoError(t, err)

		res, err := ListApps(t.Context(), dockerCli, ListAppRequest{
			ShowApps: true,
		}, idProvider, nil, cfg)
		require.NoError(t, err)

		for _, a := range res.Apps {
			assert.NotEqual(t, tmpAppName, a.Name, ".temp_ app should be ignored")

			if strings.Contains(a.ID.String(), tmpAppName) {
				t.Errorf("the app %s was not filtered out", a.Name)
			}
		}

		// check on broken apps
		for _, b := range res.BrokenApps {
			assert.NotContains(t, b.Name, tmpAppName, "the temporary app should not be in the broken apps list")
		}
	})

	t.Run("test nested apps do not produce bogus broken apps in parent dirs", func(t *testing.T) {
		// https://github.com/arduino/arduino-app-cli/issues/220

		createApp(t, "inside", false, idProvider, cfg)
		outsideAppDir := cfg.AppsDir().Join("inside")
		t.Cleanup(func() { _ = outsideAppDir.RemoveAll() })

		nestedAppDir := cfg.AppsDir().Join("nested")
		require.NoError(t, cfg.AppsDir().Join("nested").Mkdir())
		t.Cleanup(func() { _ = nestedAppDir.RemoveAll() })
		require.NoError(t, outsideAppDir.Rename(nestedAppDir.Join("inside")))

		res, err := ListApps(t.Context(), dockerCli, ListAppRequest{
			ShowApps: true,
		}, idProvider, nil, cfg)
		require.NoError(t, err)
		require.Empty(t, res.BrokenApps)
	})
}

func TestListAppsFiltersByBricksIndex(t *testing.T) {
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

	// Create a compatible example (uses arduino:compatible_brick)
	compatibleExID := createApp(t, "compatible-example", true, idProvider, cfg)
	compatibleEx, err := app.Load(compatibleExID.ToPath())
	require.NoError(t, err)
	compatibleEx.Descriptor.Bricks = []app.Brick{{ID: "arduino:compatible_brick"}}
	require.NoError(t, compatibleEx.Save())

	// Create an incompatible example (uses arduino:incompatible_brick, absent from the index)
	incompatibleExID := createApp(t, "incompatible-example", true, idProvider, cfg)
	incompatibleEx, err := app.Load(incompatibleExID.ToPath())
	require.NoError(t, err)
	incompatibleEx.Descriptor.Bricks = []app.Brick{{ID: "arduino:incompatible_brick"}}
	require.NoError(t, incompatibleEx.Save())

	// Create a user app with the incompatible brick — should never be filtered
	userAppID := createApp(t, "user-app", false, idProvider, cfg)
	userApp, err := app.Load(userAppID.ToPath())
	require.NoError(t, err)
	userApp.Descriptor.Bricks = []app.Brick{{ID: "arduino:incompatible_brick"}}
	require.NoError(t, userApp.Save())

	// Build a bricks index that only contains arduino:compatible_brick
	bricksIndexContent := []byte(`
bricks:
- id: arduino:compatible_brick
  name: Compatible Brick
  description: A brick compatible with the selected board
`)
	require.NoError(t, cfg.AssetsDir().MkdirAll())
	require.NoError(t, cfg.AssetsDir().Join("bricks-list.yaml").WriteFile(bricksIndexContent))
	idx, err := bricksindex.Load(platform.GetPlatform(nil), cfg.AssetsDir())
	require.NoError(t, err)

	t.Run("compatible example is listed", func(t *testing.T) {
		res, err := ListApps(t.Context(), dockerCli, ListAppRequest{ShowExamples: true}, idProvider, idx, cfg)
		require.NoError(t, err)
		require.Len(t, res.Apps, 1)
		assert.Equal(t, compatibleExID, res.Apps[0].ID)
	})

	t.Run("incompatible example is excluded", func(t *testing.T) {
		res, err := ListApps(t.Context(), dockerCli, ListAppRequest{ShowExamples: true}, idProvider, idx, cfg)
		require.NoError(t, err)
		for _, a := range res.Apps {
			assert.NotEqual(t, incompatibleExID, a.ID, "incompatible example should be filtered out")
		}
	})

	t.Run("user app with incompatible brick is still listed", func(t *testing.T) {
		res, err := ListApps(t.Context(), dockerCli, ListAppRequest{ShowApps: true}, idProvider, idx, cfg)
		require.NoError(t, err)
		require.Len(t, res.Apps, 1)
		assert.Equal(t, userAppID, res.Apps[0].ID)
	})

	t.Run("nil bricks index disables filtering", func(t *testing.T) {
		res, err := ListApps(t.Context(), dockerCli, ListAppRequest{ShowExamples: true}, idProvider, nil, cfg)
		require.NoError(t, err)
		assert.Len(t, res.Apps, 2)
	})
}

func TestListAppsLocalBricksCompatibility(t *testing.T) {
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

	// Create an example that references a local brick not in the built-in index
	exampleID := createApp(t, "local-brick-example", true, idProvider, cfg)
	exampleApp, err := app.Load(exampleID.ToPath())
	require.NoError(t, err)
	exampleApp.Descriptor.Bricks = []app.Brick{{ID: "local:my_custom_brick"}}
	require.NoError(t, exampleApp.Save())

	// Add a local brick to the app's bricks/ folder
	localBrickDir := exampleID.ToPath().Join("bricks", "local", "my_custom_brick")
	require.NoError(t, localBrickDir.MkdirAll())
	localBrickConfig := []byte("id: local:my_custom_brick\nname: My Custom Brick\ndescription: A local brick\n")
	require.NoError(t, localBrickDir.Join("brick_config.yaml").WriteFile(localBrickConfig))

	// Build a bricks index with no built-in bricks (empty)
	bricksIndexContent := []byte("bricks: []\n")
	require.NoError(t, cfg.AssetsDir().MkdirAll())
	require.NoError(t, cfg.AssetsDir().Join("bricks-list.yaml").WriteFile(bricksIndexContent))
	idx, err := bricksindex.Load(platform.GetPlatform(nil), cfg.AssetsDir())
	require.NoError(t, err)

	t.Run("example with only local bricks is listed even when index is empty", func(t *testing.T) {
		res, err := ListApps(t.Context(), dockerCli, ListAppRequest{ShowExamples: true}, idProvider, idx, cfg)
		require.NoError(t, err)
		require.Len(t, res.Apps, 1)
		assert.Equal(t, exampleID, res.Apps[0].ID)
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
  require_model: true
  ports: []
  category: video
  model_name: yolox-object-detection
  variables:
  - name: CUSTOM_MODEL_PATH
    default_value: /home/arduino/.arduino-bricks/models
    description: path to the custom model directory
  - name: EI_OBJ_DETECTION_MODEL
    default_value: /models/ootb/ei/yolo-x-nano.eim
    description: path to the model file
`)
	err = cfg.AssetsDir().Join("bricks-list.yaml").WriteFile(bricksIndexContent)
	require.NoError(t, err)
	bricksIndex, err := bricksindex.Load(platform.GetPlatform(nil), cfg.AssetsDir())
	assert.NoError(t, err)

	modelsIndexContent := []byte(`
models:
- yolox-object-detection:
    runner: brick
    name : "General purpose object detection - YoloX"
    description: "General purpose object detection model based on YoloX Nano. This model is trained on the COCO dataset and can detect 80 different object classes."
    metadata:
      source: "edgeimpulse"
      ei-project-id: 717280
      source-model-id: "YOLOX-Nano"
      source-model-url: "https://github.com/Megvii-BaseDetection/YOLOX"
    bricks:
    - id: arduino:object_detection
    - id: arduino:video_object_detection
`)
	err = cfg.AssetsDir().Join("models-list.yaml").WriteFile(modelsIndexContent)
	require.NoError(t, err)
	modelIndex, err := modelsindex.Load(platform.GetPlatform(nil), cfg.AssetsDir(), nil)
	require.NoError(t, err)

	env := getAppEnvironmentVariables(appDesc, bricksIndex, modelIndex)
	require.Equal(t, cfg.AppsDir().Join("app1").String(), env["APP_HOME"])
	require.Equal(t, "/models/ootb/ei/yolo-x-nano.eim", env["EI_OBJ_DETECTION_MODEL"])
	require.Equal(t, "/home/arduino/.arduino-bricks/models", env["CUSTOM_MODEL_PATH"])
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
				"EI_OBJ_DETECTION_MODEL": "/home/arduino/.arduino-bricks/models/face-det.eim",
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
  require_model: true
  category: video
  model_name: yolox-object-detection
  variables:
  - name: CUSTOM_MODEL_PATH
    default_value: /home/arduino/.arduino-bricks/models
    description: path to the custom model directory
  - name: EI_OBJ_DETECTION_MODEL
    default_value: /models/ootb/ei/yolo-x-nano.eim
    description: path to the model file
`)
	err = cfg.AssetsDir().Join("bricks-list.yaml").WriteFile(bricksIndexContent)
	require.NoError(t, err)
	bricksIndex, err := bricksindex.Load(platform.GetPlatform(nil), cfg.AssetsDir())
	assert.NoError(t, err)

	modelsIndexContent := []byte(`
models:
- yolox-object-detection:
    runner: brick
    name : "General purpose object detection - YoloX"
    description: "General purpose object detection model based on YoloX Nano. This model is trained on the COCO dataset and can detect 80 different object classes."
    metadata:
      source: "edgeimpulse"
      ei-project-id: 717280
      source-model-id: "YOLOX-Nano"
      source-model-url: "https://github.com/Megvii-BaseDetection/YOLOX"
    bricks:
    - id: arduino:object_detection
    - id: arduino:video_object_detection
`)
	err = cfg.AssetsDir().Join("models-list.yaml").WriteFile(modelsIndexContent)
	require.NoError(t, err)
	modelIndex, err := modelsindex.Load(platform.GetPlatform(nil), cfg.AssetsDir(), nil)
	require.NoError(t, err)

	env := getAppEnvironmentVariables(appDesc, bricksIndex, modelIndex)
	require.Equal(t, cfg.AppsDir().Join("app1").String(), env["APP_HOME"])
	require.Equal(t, "/home/arduino/.arduino-bricks/models/face-det.eim", env["EI_OBJ_DETECTION_MODEL"])
	require.Equal(t, "/home/arduino/.arduino-bricks/models", env["CUSTOM_MODEL_PATH"])
	// we ignore HOST_IP since it's dynamic
}

func TestGetAppEnvironmentVariablesUsingMultipleBricks(t *testing.T) {
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
		{ID: "arduino:object_detection", Model: "a-model-compatible-with-multiple-bricks"},
		{ID: "arduino:video_object_detection", Model: "a-model-compatible-with-multiple-bricks"},
	}

	bricksIndexContent := []byte(`
bricks:
  - id: arduino:object_detection
    model_name: a-model-compatible-with-multiple-bricks
    variables:
      - name: EI_OBJ_DETECTION_MODEL
        description: Path to the model file
        hidden: true
        default_value: /default/path/obj.eim
      - name: COMMON_ENV
        description: a common env variable between bricks
        default_value: "default-common-video"

  - id: arduino:video_object_detection
    model_name: a-model-compatible-with-multiple-bricks
    variables:
      - name: EI_V_OBJ_DETECTION_MODEL
        description: Path to the model file
        hidden: true
        default_value: /default/path/video.eim
      - name: COMMON_ENV
        description: a common env variable between bricks
        default_value: "default-common-obj"
      - name: MY_VIDEO_ENV
        description: Video device path
        hidden: true
        default_value: /default/video/value

  `)
	err = cfg.AssetsDir().Join("bricks-list.yaml").WriteFile(bricksIndexContent)
	require.NoError(t, err)
	bricksIndex, err := bricksindex.Load(platform.GetPlatform(nil), cfg.AssetsDir())
	assert.NoError(t, err)

	modelsIndexContent := []byte(`
models:
  - a-model-compatible-with-multiple-bricks:
      bricks:
        - id: arduino:object_detection
          model_configuration:
            EI_OBJ_DETECTION_MODEL: "/models/path/obj.eim"
        - id: arduino:video_object_detection
          model_configuration:
            EI_V_OBJ_DETECTION_MODEL: "/models/path/video.eim"
`)
	err = cfg.AssetsDir().Join("models-list.yaml").WriteFile(modelsIndexContent)
	require.NoError(t, err)
	modelIndex, err := modelsindex.Load(platform.GetPlatform(nil), cfg.AssetsDir(), nil)
	require.NoError(t, err)

	env := getAppEnvironmentVariables(appDesc, bricksIndex, modelIndex)
	require.Equal(t, "/models/path/obj.eim", env["EI_OBJ_DETECTION_MODEL"])
	require.Equal(t, "/models/path/video.eim", env["EI_V_OBJ_DETECTION_MODEL"])
	require.Equal(t, "/default/video/value", env["MY_VIDEO_ENV"])
	// for common env variable, the last brick wins
	require.Equal(t, "default-common-obj", env["COMMON_ENV"])
}
