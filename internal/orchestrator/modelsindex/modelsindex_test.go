package modelsindex

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateModelsIndexFromFile(t *testing.T) {
	t.Run("it parses a valid model-list.yaml", func(t *testing.T) {
		modelsIndex, err := GenerateModelsIndexFromFile(paths.New("testdata"))
		require.NoError(t, err)
		require.NotNil(t, modelsIndex)

		models := modelsIndex.GetModels()
		assert.Len(t, models, 2, "Expected 2 models to be parsed")
	})

	t.Run("it gets a model by ID", func(t *testing.T) {
		modelsIndex, err := GenerateModelsIndexFromFile(paths.New("testdata"))
		require.NoError(t, err)

		model, found := modelsIndex.GetModelByID("not-existing-model")
		assert.False(t, found)
		assert.Nil(t, model)

		model, found = modelsIndex.GetModelByID("face-detection")
		assert.Equal(t, "brick", model.Runner)
		require.True(t, found, "face-detection should be found")
		assert.Equal(t, "face-detection", model.ID)
		assert.Equal(t, "Lightweight-Face-Detection", model.Name)
		assert.Equal(t, "Face bounding box detection. This model is trained on the WIDER FACE dataset and can detect faces in images.", model.ModuleDescription)
		assert.Equal(t, []string{"face"}, model.ModelLabels)
		assert.Equal(t, "/models/ootb/ei/lw-face-det.eim", model.ModelConfiguration["EI_OBJ_DETECTION_MODEL"])
		assert.Equal(t, []string{"arduino:object_detection", "arduino:video_object_detection"}, model.Bricks)
		assert.Equal(t, "qualcomm-ai-hub", model.Metadata["source"])
		assert.Equal(t, "false", model.Metadata["ei-gpu-mode"])
		assert.Equal(t, "face-det-lite", model.Metadata["source-model-id"])
		assert.Equal(t, "https://aihub.qualcomm.com/models/face_det_lite", model.Metadata["source-model-url"])
	})

	t.Run("it fails if model-list.yaml does not exist", func(t *testing.T) {
		nonExistentPath := paths.New("nonexistent.yaml")
		modelsIndex, err := GenerateModelsIndexFromFile(nonExistentPath)
		assert.Error(t, err)
		assert.Nil(t, modelsIndex)
	})

	t.Run("it filters models by a single brick", func(t *testing.T) {
		modelsIndex, err := GenerateModelsIndexFromFile(paths.New("testdata"))
		require.NoError(t, err)

		brick1Models := modelsIndex.GetModelsByBrick("arduino:object_detection")
		assert.Len(t, brick1Models, 1)
		assert.Equal(t, "face-detection", brick1Models[0].ID)

		brick1Models = modelsIndex.GetModelsByBrick("not-existing-brick")
		assert.Nil(t, brick1Models)
	})

	t.Run("it filters models by multiple bricks", func(t *testing.T) {
		modelsIndex, err := GenerateModelsIndexFromFile(paths.New("testdata"))
		require.NoError(t, err)

		brick2Models := modelsIndex.GetModelsByBrick("arduino:video_object_detection")
		assert.Len(t, brick2Models, 2)
		assert.Equal(t, "face-detection", brick2Models[0].ID)
		assert.Equal(t, "yolox-object-detection", brick2Models[1].ID)

		bricks2Models := modelsIndex.GetModelsByBricks([]string{"arduino:object_detection", "arduino:video_object_detection"})
		assert.Len(t, bricks2Models, 2)
		assert.Equal(t, "face-detection", bricks2Models[0].ID)
		assert.Equal(t, "yolox-object-detection", bricks2Models[1].ID)

		nonExistentModels := modelsIndex.GetModelsByBrick("nonexistent_brick")
		assert.Nil(t, nonExistentModels)
	})
}
