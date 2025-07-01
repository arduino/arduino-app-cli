package orchestrator

import (
	"fmt"
	"testing"
	"time"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"

	"github.com/stretchr/testify/require"
	"go.bug.st/f"
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
		require.Equal(t, ID("user/example-app"), r.ID)

		t.Run("skip python", func(t *testing.T) {
			r, err := CreateApp(t.Context(), CreateAppRequest{
				Name:       "skip-python",
				SkipPython: true,
			})
			require.NoError(t, err)
			require.Equal(t, ID("user/skip-python"), r.ID)
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
			require.Equal(t, ID("user/skip-sketch"), r.ID)
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

	originalAppID := f.Must(ParseID("user/original-app"))
	originalAppPath := originalAppID.ToPath()
	r, err := CreateApp(t.Context(), CreateAppRequest{Name: "original-app"})
	require.NoError(t, err)
	require.Equal(t, originalAppID, r.ID)
	require.DirExists(t, originalAppPath.String())

	t.Run("valid clone", func(t *testing.T) {
		t.Run("without name", func(t *testing.T) {
			resp, err := CloneApp(t.Context(), CloneAppRequest{FromID: originalAppID})
			require.NoError(t, err)
			require.Equal(t, ID("user/original-app-copy0"), resp.ID)
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
			require.Equal(t, ID("user/new-name"), resp.ID)
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
			require.Equal(t, ID("user/with-icon"), resp.ID)
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

			resp, err := CloneApp(t.Context(), CloneAppRequest{FromID: ID("user/app-with-cache")})
			require.NoError(t, err)
			require.Equal(t, ID("user/app-with-cache-copy0"), resp.ID)
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
			_, err := CloneApp(t.Context(), CloneAppRequest{FromID: ID("user/not-existing")})
			require.ErrorIs(t, err, ErrAppDoesntExists)
		})
		t.Run("missing app yaml", func(t *testing.T) {
			err := orchestratorConfig.appsDir.Join("app-without-yaml").Mkdir()
			require.NoError(t, err)
			_, err = CloneApp(t.Context(), CloneAppRequest{FromID: ID("user/app-without-yaml")})
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

	createAppWithBricks := func(t *testing.T, bricks []app.Brick) *app.ArduinoApp {
		t.Helper()
		name := fmt.Sprintf("app-%v", time.Now().UnixNano())
		_, err := CreateApp(t.Context(), CreateAppRequest{Name: name})
		require.NoError(t, err)
		appWithBricksDir := orchestratorConfig.AppsDir().Join(name)
		appWithBricks := f.Ptr(f.Must(app.Load(appWithBricksDir.String())))
		appWithBricks.Descriptor.Bricks = bricks
		require.NoError(t, err)
		err = appWithBricks.Save()
		require.NoError(t, err)
		return appWithBricks
	}

	t.Run("with brick variables", func(t *testing.T) {
		t.Run("add new brick", func(t *testing.T) {
			appWithBricks := createAppWithBricks(t, []app.Brick{})
			err := EditApp(AppEditRequest{
				Default: new(bool),
				Variables: f.Ptr(map[string]map[string]string{
					"arduino:object_detection": {"CUSTOM_MODEL_PATH": "/opt/models/ei"},
				}),
			}, appWithBricks)
			require.NoError(t, err)
		})

		t.Run("override variables to existing brick", func(t *testing.T) {
			appWithBricks := createAppWithBricks(t, []app.Brick{
				{
					ID:        "arduino:object_detection",
					Variables: map[string]string{"CUSTOM_MODEL_PATH": "/opt/models/ei"},
				},
			})

			newVariables := map[string]map[string]string{
				"arduino:object_detection": {"CUSTOM_MODEL_PATH": "/new"},
			}
			err := EditApp(AppEditRequest{
				Default:   new(bool),
				Variables: &newVariables,
			}, appWithBricks)
			require.NoError(t, err)

			newApp, err := app.Load(appWithBricks.FullPath.String())
			require.NoError(t, err)
			require.Len(t, newApp.Descriptor.Bricks, 1)
			require.Equal(t, "arduino:object_detection", newApp.Descriptor.Bricks[0].ID)
			require.Equal(t, newVariables["arduino:object_detection"], newApp.Descriptor.Bricks[0].Variables)
		})
		t.Run("setting not existing variable", func(t *testing.T) {
			appWithBricks := createAppWithBricks(t, []app.Brick{})

			newVariables := map[string]map[string]string{
				"arduino:object_detection": {"NOT_EXISTING_VAR": "nope"},
			}
			err := EditApp(AppEditRequest{
				Default:   new(bool),
				Variables: &newVariables,
			}, appWithBricks)
			require.Error(t, err)

			newApp, err := app.Load(appWithBricks.FullPath.String())
			require.NoError(t, err)
			require.Len(t, newApp.Descriptor.Bricks, 0)
		})
	})
}

func setTestOrchestratorConfig(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	t.Setenv("ARDUINO_APP_CLI__APPS_DIR", tmpDir)
	t.Setenv("ARDUINO_APP_CLI__DATA_DIR", tmpDir)
	cfg, err := NewOrchestratorConfigFromEnv()
	require.NoError(t, err)

	// Override the global config with the test one
	orchestratorConfig = cfg
}
