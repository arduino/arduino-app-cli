package orchestrator

import (
	"testing"

	"github.com/arduino/arduino-app-cli/pkg/parser"

	"github.com/stretchr/testify/require"
	"go.bug.st/f"
)

func TestCreateApp(t *testing.T) {
	setTestOrchestratorConfig(t)

	t.Run("valid app", func(t *testing.T) {
		r, err := CreateApp(t.Context(), CreateAppRequest{
			Name:   "example app",
			Icon:   "😃",
			Bricks: []string{"arduino/object-detection"},
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

	originalAppID := ID("user/original-app")
	originalAppPath := f.Must(originalAppID.ToPath())
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
			appDir := f.Must(resp.ID.ToPath())
			require.DirExists(t, appDir.String())
			t.Cleanup(func() {
				_ = appDir.RemoveAll()
			})

			// The app.yaml will have the display-name set to the new-name
			clonedApp := f.Must(parser.Load(appDir.String()))
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
			appDir := f.Must(resp.ID.ToPath())
			require.DirExists(t, appDir.String())
			t.Cleanup(func() {
				_ = appDir.RemoveAll()
			})

			// The app.yaml will have the icon set to 🦄
			clonedApp := f.Must(parser.Load(appDir.String()))
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
			appDir := f.Must(resp.ID.ToPath())
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
