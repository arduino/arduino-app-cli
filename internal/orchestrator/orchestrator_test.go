package orchestrator

import (
	"fmt"
	"os"
	"testing"

	"github.com/arduino/go-paths-helper"
	dockerClient "github.com/docker/docker/client"
	gCmp "github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

func TestCreateApp(t *testing.T) {
	setTestOrchestratorConfig(t)
	t.Run("valid app", func(t *testing.T) {
		r, err := CreateApp(t.Context(), CreateAppRequest{
			Name:   "example app",
			Icon:   "😃",
			Bricks: []string{"arduino:object-detection"},
		})
		require.NoError(t, err)
		require.Equal(t, f.Must(ParseID("user:example-app")), r.ID)

		t.Run("skip python", func(t *testing.T) {
			r, err := CreateApp(t.Context(), CreateAppRequest{
				Name:       "skip-python",
				SkipPython: true,
			})
			require.NoError(t, err)
			require.Equal(t, f.Must(ParseID("user:skip-python")), r.ID)
			appDir := orchestratorConfig.AppsDir().Join("skip-python")
			require.DirExists(t, appDir.String())
			require.NoDirExists(t, appDir.Join("python").String())
			require.FileExists(t, appDir.Join("sketch", "sketch.ino").String())
			require.FileExists(t, appDir.Join("sketch", "sketch.yaml").String())
		})
		t.Run("skip sketch", func(t *testing.T) {
			r, err := CreateApp(t.Context(), CreateAppRequest{
				Name:       "skip-sketch",
				SkipSketch: true,
			})
			require.NoError(t, err)
			require.Equal(t, f.Must(ParseID("user:skip-sketch")), r.ID)
			appDir := orchestratorConfig.AppsDir().Join("skip-sketch")
			require.DirExists(t, appDir.String())
			require.NoDirExists(t, appDir.Join("sketch").String())
		})
	})

	t.Run("invalid app", func(t *testing.T) {
		t.Run("empty name", func(t *testing.T) {
			_, err := CreateApp(t.Context(), CreateAppRequest{Name: ""})
			require.Error(t, err)
		})
		t.Run("app already present", func(t *testing.T) {
			r := CreateAppRequest{Name: "present"}
			_, err := CreateApp(t.Context(), r)
			require.NoError(t, err)
			_, err = CreateApp(t.Context(), r)
			require.ErrorIs(t, err, ErrAppAlreadyExists)
		})
		t.Run("skipping both python and sketch", func(t *testing.T) {
			_, err := CreateApp(t.Context(), CreateAppRequest{
				Name:       "skip-both",
				SkipPython: true,
				SkipSketch: true,
			})
			require.Error(t, err)
		})
	})
}

func TestCloneApp(t *testing.T) {
	setTestOrchestratorConfig(t)

	originalAppID := f.Must(ParseID("user:original-app"))
	originalAppPath := originalAppID.ToPath()
	r, err := CreateApp(t.Context(), CreateAppRequest{Name: "original-app"})
	require.NoError(t, err)
	require.Equal(t, originalAppID, r.ID)
	require.DirExists(t, originalAppPath.String())

	t.Run("valid clone", func(t *testing.T) {
		t.Run("without name", func(t *testing.T) {
			resp, err := CloneApp(t.Context(), CloneAppRequest{FromID: originalAppID})
			require.NoError(t, err)
			require.Equal(t, f.Must(ParseID("user:original-app-copy0")), resp.ID)
			appDir := orchestratorConfig.AppsDir().Join("original-app-copy0")
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
			})
			require.NoError(t, err)
			require.Equal(t, f.Must(ParseID("user:new-name")), resp.ID)
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
			})
			require.NoError(t, err)
			require.Equal(t, f.Must(ParseID("user:with-icon")), resp.ID)
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
			baseApp := orchestratorConfig.appsDir.Join("app-with-cache")
			require.NoError(t, baseApp.Join(".cache").MkdirAll())
			require.NoError(t, baseApp.Join("data").MkdirAll())
			require.NoError(t, baseApp.Join("app.yaml").WriteFile([]byte("name: app-with-cache")))

			resp, err := CloneApp(t.Context(), CloneAppRequest{FromID: f.Must(ParseID("user:app-with-cache"))})
			require.NoError(t, err)
			require.Equal(t, f.Must(ParseID("user:app-with-cache-copy0")), resp.ID)
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
			_, err := CloneApp(t.Context(), CloneAppRequest{FromID: f.Must(ParseID("user:not-existing"))})
			require.ErrorIs(t, err, ErrAppDoesntExists)
		})
		t.Run("missing app yaml", func(t *testing.T) {
			err := orchestratorConfig.appsDir.Join("app-without-yaml").Mkdir()
			require.NoError(t, err)
			_, err = CloneApp(t.Context(), CloneAppRequest{FromID: f.Must(ParseID("user:app-without-yaml"))})
			require.ErrorIs(t, err, ErrInvalidApp)
		})
		t.Run("name already exists", func(t *testing.T) {
			_, err = CloneApp(t.Context(), CloneAppRequest{
				FromID: originalAppID,
				Name:   f.Ptr("original-app"),
			})
			require.ErrorIs(t, err, ErrAppAlreadyExists)
		})
	})
}

func TestEditApp(t *testing.T) {
	setTestOrchestratorConfig(t)

	t.Run("with default", func(t *testing.T) {
		_, err := CreateApp(t.Context(), CreateAppRequest{Name: "app-default"})
		require.NoError(t, err)
		appDir := orchestratorConfig.AppsDir().Join("app-default")

		t.Run("previously not default", func(t *testing.T) {
			app := f.Must(app.Load(appDir.String()))

			previousDefaultApp, err := GetDefaultApp()
			require.NoError(t, err)
			require.Nil(t, previousDefaultApp)

			err = EditApp(AppEditRequest{Default: f.Ptr(true)}, &app)
			require.NoError(t, err)

			currentDefaultApp, err := GetDefaultApp()
			require.NoError(t, err)
			require.True(t, appDir.EquivalentTo(currentDefaultApp.FullPath))
		})
		t.Run("previously default", func(t *testing.T) {
			app := f.Must(app.Load(appDir.String()))
			err := SetDefaultApp(&app)
			require.NoError(t, err)

			previousDefaultApp, err := GetDefaultApp()
			require.NoError(t, err)
			require.True(t, appDir.EquivalentTo(previousDefaultApp.FullPath))

			err = EditApp(AppEditRequest{Default: f.Ptr(false)}, &app)
			require.NoError(t, err)

			currentDefaultApp, err := GetDefaultApp()
			require.NoError(t, err)
			require.Nil(t, currentDefaultApp)
		})
	})

	t.Run("with name", func(t *testing.T) {
		originalAppName := "original-name"
		_, err := CreateApp(t.Context(), CreateAppRequest{Name: originalAppName})
		require.NoError(t, err)
		appDir := orchestratorConfig.AppsDir().Join(originalAppName)
		originalApp := f.Must(app.Load(appDir.String()))

		err = EditApp(AppEditRequest{Name: f.Ptr("new-name")}, &originalApp)
		require.NoError(t, err)
		editedApp, err := app.Load(orchestratorConfig.AppsDir().Join("new-name").String())
		require.NoError(t, err)
		require.Equal(t, "new-name", editedApp.Name)
		require.True(t, originalApp.FullPath.NotExist()) // The original app directory should be removed after renaming

		t.Run("already existing name", func(t *testing.T) {
			existingAppName := "existing-name"
			_, err := CreateApp(t.Context(), CreateAppRequest{Name: existingAppName})
			require.NoError(t, err)
			appDir := orchestratorConfig.AppsDir().Join(existingAppName)
			existingApp := f.Must(app.Load(appDir.String()))

			err = EditApp(AppEditRequest{Name: f.Ptr(existingAppName)}, &existingApp)
			require.ErrorIs(t, err, ErrAppAlreadyExists)
		})
	})

	t.Run("with icon and description", func(t *testing.T) {
		commonAppName := "common-app"
		_, err := CreateApp(t.Context(), CreateAppRequest{Name: commonAppName})
		require.NoError(t, err)
		commonAppDir := orchestratorConfig.AppsDir().Join(commonAppName)
		commonApp := f.Must(app.Load(commonAppDir.String()))

		err = EditApp(AppEditRequest{
			Icon:        f.Ptr("💻"),
			Description: f.Ptr("new desc"),
		}, &commonApp)
		require.NoError(t, err)
		editedApp := f.Must(app.Load(commonAppDir.String()))
		require.Equal(t, "new desc", editedApp.Descriptor.Description)
		require.Equal(t, "💻", editedApp.Descriptor.Icon)
	})

	// TODO: Re-enable this tests when we refactor the brick endpoint
	// 	createAppWithBricks := func(t *testing.T, bricks []app.Brick) *app.ArduinoApp {
	// 	t.Helper()
	// 	name := fmt.Sprintf("app-%v", time.Now().UnixNano())
	// 	_, err := CreateApp(t.Context(), CreateAppRequest{Name: name})
	// 	require.NoError(t, err)
	// 	appWithBricksDir := orchestratorConfig.AppsDir().Join(name)
	// 	appWithBricks := f.Ptr(f.Must(app.Load(appWithBricksDir.String())))
	// 	appWithBricks.Descriptor.Bricks = bricks
	// 	require.NoError(t, err)
	// 	err = appWithBricks.Save()
	// 	require.NoError(t, err)
	// 	return appWithBricks
	// }
	//
	// t.Run("with brick variables", func(t *testing.T) {
	// 	t.Run("add new brick", func(t *testing.T) {
	// 		appWithBricks := createAppWithBricks(t, []app.Brick{})
	// 		err := EditApp(AppEditRequest{
	// 			Default: new(bool),
	// 			Variables: f.Ptr(map[string]map[string]string{
	// 				"arduino:object_detection": {"CUSTOM_MODEL_PATH": "/opt/models/ei"},
	// 			}),
	// 		}, appWithBricks)
	// 		require.NoError(t, err)
	// 	})
	//
	// 	t.Run("override variables to existing brick", func(t *testing.T) {
	// 		appWithBricks := createAppWithBricks(t, []app.Brick{
	// 			{
	// 				ID:        "arduino:object_detection",
	// 				Variables: map[string]string{"CUSTOM_MODEL_PATH": "/opt/models/ei"},
	// 			},
	// 		})
	//
	// 		newVariables := map[string]map[string]string{
	// 			"arduino:object_detection": {"CUSTOM_MODEL_PATH": "/new"},
	// 		}
	// 		err := EditApp(AppEditRequest{
	// 			Default:   new(bool),
	// 			Variables: &newVariables,
	// 		}, appWithBricks)
	// 		require.NoError(t, err)
	//
	// 		newApp, err := app.Load(appWithBricks.FullPath.String())
	// 		require.NoError(t, err)
	// 		require.Len(t, newApp.Descriptor.Bricks, 1)
	// 		require.Equal(t, "arduino:object_detection", newApp.Descriptor.Bricks[0].ID)
	// 		require.Equal(t, newVariables["arduino:object_detection"], newApp.Descriptor.Bricks[0].Variables)
	// 	})
	// 	t.Run("setting not existing variable", func(t *testing.T) {
	// 		appWithBricks := createAppWithBricks(t, []app.Brick{})
	//
	// 		newVariables := map[string]map[string]string{
	// 			"arduino:object_detection": {"NOT_EXISTING_VAR": "nope"},
	// 		}
	// 		err := EditApp(AppEditRequest{
	// 			Default:   new(bool),
	// 			Variables: &newVariables,
	// 		}, appWithBricks)
	// 		require.Error(t, err)
	//
	// 		newApp, err := app.Load(appWithBricks.FullPath.String())
	// 		require.NoError(t, err)
	// 		require.Len(t, newApp.Descriptor.Bricks, 0)
	// 	})
	// })
}

func TestListApp(t *testing.T) {
	setTestOrchestratorConfig(t)

	docker, err := dockerClient.NewClientWithOpts(
		dockerClient.FromEnv,
		dockerClient.WithAPIVersionNegotiation(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { docker.Close() })

	createApp(t, "app1", false)
	createApp(t, "app2", false)
	createApp(t, "example1", true)

	t.Run("list all apps", func(t *testing.T) {
		res, err := ListApps(t.Context(), docker, ListAppRequest{
			ShowApps:     true,
			ShowExamples: true,
			StatusFilter: "",
		})
		require.NoError(t, err)
		// FIXME: we should enable this assertion when we have broken apps
		// assert.Empty(t, res.BrokenApps)
		assert.Empty(t, gCmp.Diff([]AppInfo{
			{
				ID:          f.Must(ParseID("examples:example1")),
				Name:        "example1",
				Description: "",
				Icon:        "😃",
				Status:      "",
				Example:     true,
				Default:     false,
			},
			{
				ID:          f.Must(ParseID("user:app1")),
				Name:        "app1",
				Description: "",
				Icon:        "😃",
				Status:      "",
				Example:     false,
				Default:     false,
			},
			{
				ID:          f.Must(ParseID("user:app2")),
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
		res, err := ListApps(t.Context(), docker, ListAppRequest{
			ShowApps:     true,
			ShowExamples: false,
			StatusFilter: "",
		})
		require.NoError(t, err)
		// FIXME: we should enable this assertion when we have broken apps
		// assert.Empty(t, res.BrokenApps)
		assert.Empty(t, gCmp.Diff([]AppInfo{
			{
				ID:          f.Must(ParseID("user:app1")),
				Name:        "app1",
				Description: "",
				Icon:        "😃",
				Status:      "",
				Example:     false,
				Default:     false,
			},
			{
				ID:          f.Must(ParseID("user:app2")),
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
		res, err := ListApps(t.Context(), docker, ListAppRequest{
			ShowApps:     false,
			ShowExamples: true,
			StatusFilter: "",
		})
		require.NoError(t, err)
		// FIXME: we should enable this assertion when we have broken apps
		// assert.Empty(t, res.BrokenApps)
		assert.Empty(t, gCmp.Diff([]AppInfo{
			{
				ID:          f.Must(ParseID("examples:example1")),
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

func TestAppDetails(t *testing.T) {
	setTestOrchestratorConfig(t)

	docker, err := dockerClient.NewClientWithOpts(
		dockerClient.FromEnv,
		dockerClient.WithAPIVersionNegotiation(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { docker.Close() })

	createApp(t, "app1", false)
	createApp(t, "example1", true)

	t.Run("app details", func(t *testing.T) {
		// TODO: fix data race in docker 😅
		// t.Parallel()

		id := f.Must(ParseID("user:app1"))
		app, err := app.Load(id.ToPath().String())
		require.NoError(t, err)
		details, err := AppDetails(t.Context(), docker, app)
		require.NoError(t, err)
		fmt.Println(details)
		assert.Empty(t, gCmp.Diff(details, AppDetailedInfo{
			ID:          id,
			Name:        "app1",
			Path:        orchestratorConfig.AppsDir().Join("app1").String(),
			Description: "",
			Icon:        "😃",
			Status:      "",
			Example:     false,
			Default:     false,
			Bricks:      []AppDetailedBrick{},
		}))
	})

	t.Run("example details", func(t *testing.T) {
		// TODO: fix data race in docker 😅
		// t.Parallel()

		id := f.Must(ParseID("examples:example1"))
		app, err := app.Load(id.ToPath().String())
		require.NoError(t, err)
		details, err := AppDetails(t.Context(), docker, app)
		require.NoError(t, err)
		assert.Empty(t, gCmp.Diff(details, AppDetailedInfo{
			ID:          id,
			Name:        "example1",
			Path:        orchestratorConfig.ExamplesDir().Join("example1").String(),
			Description: "",
			Icon:        "😃",
			Status:      "",
			Example:     true,
			Default:     false,
			Bricks:      []AppDetailedBrick{},
		}))
	})
}

func setTestOrchestratorConfig(t *testing.T) {
	t.Helper()

	tmpDir := paths.New(t.TempDir())
	t.Setenv("ARDUINO_APP_CLI__APPS_DIR", tmpDir.Join("apps").String())
	t.Setenv("ARDUINO_APP_CLI__DATA_DIR", tmpDir.Join("data").String())
	cfg, err := NewOrchestratorConfigFromEnv()
	require.NoError(t, err)

	// Override the global config with the test one
	orchestratorConfig = cfg
}

func createApp(t *testing.T, name string, isExample bool) {
	t.Helper()

	res, err := CreateApp(t.Context(), CreateAppRequest{
		Name: name,
		Icon: "😃",
	})
	require.NoError(t, err)
	require.Empty(t, gCmp.Diff(f.Must(ParseID("user:"+name)), res.ID))
	if isExample {
		newPath := orchestratorConfig.ExamplesDir().Join(name)
		err = os.Rename(res.ID.ToPath().String(), newPath.String())
		require.NoError(t, err)
		newID, err := NewIDFromPath(newPath)
		require.NoError(t, err)
		assert.Empty(t, gCmp.Diff(f.Must(ParseID("examples:"+name)), newID))
	}
}
