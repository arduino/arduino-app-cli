package orchestrator

import (
	"testing"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricks"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"
)

func TestDeleteModel(t *testing.T) {
	t.Run("delete unreferenced custom models", func(t *testing.T) {
		/* create the app */
		tempDummyApp := paths.New("testdata/dummy-app-delete-model.temp")
		err := tempDummyApp.RemoveAll()
		require.Nil(t, err)
		require.Nil(t, paths.New("testdata/dummy-app").CopyDirTo(tempDummyApp))

		/* load bricks */
		bricksIndex, err := bricksindex.Load(paths.New("testdata"))
		require.Nil(t, err)
		brickService := bricks.NewService(nil, bricksIndex, nil)

		/* load one pre-installed model and one custom model */
		modelsIndex, err := modelsindex.Load(paths.New("testdata"), paths.New("testdata/models"))
		require.NoError(t, err)
		require.NotNil(t, modelsIndex)
		models := modelsIndex.GetModels()
		assert.Len(t, models, 2, "Expected 2 models to be parsed")

		/* set model variables in app.yaml */
		modelPath := "/home/arduino/.arduino-bricks/custom-model-123/model.eim"
		modelId := "custom-classification-model-eim"
		brickId := "arduino:brick-with-custom-model"
		req := bricks.BrickCreateUpdateRequest{
			ID: brickId,
			Variables: map[string]string{
				"EI_CLASSIFICATION_MODEL": modelId,
				"CUSTOM_MODEL_PATH":       modelPath,
			},
		}
		err = brickService.BrickUpdate(req, f.Must(app.Load(tempDummyApp)))
		require.Nil(t, err)

		/* load the app to verify changes are in place */
		after, err := app.Load(tempDummyApp)
		require.Nil(t, err)
		require.Len(t, after.Descriptor.Bricks, 1)
		require.Equal(t, brickId, after.Descriptor.Bricks[0].ID)
		require.Equal(t, modelId, after.Descriptor.Bricks[0].Variables["EI_CLASSIFICATION_MODEL"])
		require.Equal(t, modelPath, after.Descriptor.Bricks[0].Variables["CUSTOM_MODEL_PATH"])

		// test model not found
		cfg, err := config.NewFromEnv()
		err = AIModelDelete(t.Context(), nil, cfg, modelsIndex, "missing-model-id", nil, false)
		assert.ErrorIs(t, err, ErrNotFound)

		// test delete a pre installed model
		err = AIModelDelete(t.Context(), nil, cfg, modelsIndex, "face-detection", nil, false)
		assert.ErrorIs(t, err, ErrConflict)
	})
}
