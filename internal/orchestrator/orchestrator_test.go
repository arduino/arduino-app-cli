package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/require"
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
