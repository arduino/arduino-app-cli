package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

func TestCreateBrickApp(t *testing.T) {
	setTestOrchestratorConfig(t)
	t.Run("valid app", func(t *testing.T) {
		r, err := CreateApp(t.Context(), CreateAppRequest{
			Name:   "example app",
			Icon:   "😃",
			Bricks: []string{"arduino:object_detection"},
		})
		require.NoError(t, err)

		require.Equal(t,
			f.Must(NewIDFromPath(orchestratorConfig.AppsDir().Join("example-app"))),
			r.ID,
		)
		require.NoError(t, err)

		appDir := r.ID.ToPath()
		require.DirExists(t, appDir.String())
		t.Cleanup(func() {
			_ = appDir.RemoveAll()
		})
		app := f.Must(app.Load(appDir.String()))
		model := "yolox-object-detection"
		brick := BrickCreateUpdateRequest{
			ID:    "arduino:object_detection",
			Model: &model,
			Variables: map[string]string{
				"CUSTOM_MODEL_PATH": " /opt/models/custom/",
			},
		}

		err = BrickCreate(brick, app)
		require.NoError(t, err)
		model = "face-detection"

		brickPathUpdate := BrickCreateUpdateRequest{
			ID: brick.ID,
			Variables: map[string]string{
				"CUSTOM_MODEL_PATH": "/opt/models/thisone/",
			},
			Model: &model,
		}

		err = BrickUpdate(brickPathUpdate, app)
		require.Equal(t, brickPathUpdate.Variables["CUSTOM_MODEL_PATH"], "/opt/models/thisone/")
		require.Equal(t, app.Descriptor.Bricks[0].Variables["CUSTOM_MODEL_PATH"], "/opt/models/thisone/")
		require.NoError(t, err)

		brickPathUpdate = BrickCreateUpdateRequest{
			ID: brick.ID,
			Variables: map[string]string{
				"notexists": "/opt/models/thisone/",
			},
		}

		err = BrickUpdate(brickPathUpdate, app)
		require.Error(t, err)

		bricks, err := BricksDetails(brickPathUpdate.ID)
		require.NoError(t, err)

		require.Equal(t, bricks.ID, brick.ID)

		err = BrickDelete(brick.ID, &app)
		require.NoError(t, err)
		require.Equal(t, len(app.Descriptor.Bricks), 0)

		require.Equal(t, bricks.ID, brick.ID)

	})
}
