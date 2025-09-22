package orchestrator

import (
	"os"
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
			clonedApp := f.Must(app.Load(appDir.String()))
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
			clonedApp := f.Must(app.Load(appDir.String()))
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
			require.ErrorIs(t, err, ErrInvalidApp)
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
			app := f.Must(app.Load(appDir.String()))

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
			app := f.Must(app.Load(appDir.String()))
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
		userApp := f.Must(app.Load(appDir.String()))
		originalPath := userApp.FullPath

		err = EditApp(AppEditRequest{Name: f.Ptr("new-name")}, &userApp, cfg)
		require.NoError(t, err)
		editedApp, err := app.Load(cfg.AppsDir().Join("new-name").String())
		require.NoError(t, err)
		require.Equal(t, "new-name", editedApp.Name)
		require.True(t, originalPath.NotExist()) // The original app directory should be removed after renaming

		t.Run("already existing name", func(t *testing.T) {
			existingAppName := "existing-name"
			_, err := CreateApp(t.Context(), CreateAppRequest{Name: existingAppName}, idProvider, cfg)
			require.NoError(t, err)
			appDir := cfg.AppsDir().Join(existingAppName)
			existingApp := f.Must(app.Load(appDir.String()))

			err = EditApp(AppEditRequest{Name: f.Ptr(existingAppName)}, &existingApp, cfg)
			require.ErrorIs(t, err, ErrAppAlreadyExists)
		})
	})

	t.Run("with icon and description", func(t *testing.T) {
		commonAppName := "common-app"
		_, err := CreateApp(t.Context(), CreateAppRequest{Name: commonAppName}, idProvider, cfg)
		require.NoError(t, err)
		commonAppDir := cfg.AppsDir().Join(commonAppName)
		commonApp := f.Must(app.Load(commonAppDir.String()))

		err = EditApp(AppEditRequest{
			Icon:        f.Ptr("💻"),
			Description: f.Ptr("new desc"),
		}, &commonApp, cfg)
		require.NoError(t, err)
		editedApp := f.Must(app.Load(commonAppDir.String()))
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
				Status:      "",
				Example:     true,
				Default:     false,
			},
			{
				ID:          f.Must(idProvider.ParseID("user:app1")),
				Name:        "app1",
				Description: "",
				Icon:        "😃",
				Status:      "",
				Example:     false,
				Default:     false,
			},
			{
				ID:          f.Must(idProvider.ParseID("user:app2")),
				Name:        "app2",
				Description: "",
				Icon:        "😃",
				Status:      "",
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
				Status:      "",
				Example:     false,
				Default:     false,
			},
			{
				ID:          f.Must(idProvider.ParseID("user:app2")),
				Name:        "app2",
				Description: "",
				Icon:        "😃",
				Status:      "",
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
				Status:      "",
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
	appDesc, err := app.Load(appId.ToPath().String())
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
  require_devices: false
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
	bricksIndex, err := bricksindex.GenerateBricksIndexFromFile(cfg.AssetsDir())
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
	modelIndex, err := modelsindex.GenerateModelsIndexFromFile(cfg.AssetsDir())
	require.NoError(t, err)

	env := getAppEnvironmentVariables(appDesc, bricksIndex, modelIndex)
	require.Equal(t, cfg.AppsDir().Join("app1").String(), env["APP_HOME"])
	require.Equal(t, "/models/ootb/ei/yolo-x-nano.eim", env["EI_OBJ_DETECTION_MODEL"])
	require.Equal(t, "/home/arduino/.arduino-bricks/ei-models", env["CUSTOM_MODEL_PATH"])
	// we ignore HOST_IP since it's dynamic
}
