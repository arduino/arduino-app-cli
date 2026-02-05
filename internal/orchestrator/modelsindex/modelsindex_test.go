package modelsindex

import (
	"path/filepath"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"
)

func TestModelsIndex(t *testing.T) {
	t.Run("it parses a valid model-list.yaml and custom models", func(t *testing.T) {
		modelsIndex, err := Load(paths.New("testdata"), paths.New("testdata/models"))
		require.NoError(t, err)
		require.NotNil(t, modelsIndex)
		models := modelsIndex.GetModels()
		assert.Len(t, models, 3, "Expected 3 models to be parsed")
	})

	t.Run("at least one model folders must be provided", func(t *testing.T) {
		_, err := Load(nil, nil)
		require.Error(t, err)
	})

	t.Run("custom models folder is optional", func(t *testing.T) {
		modelsIndex, err := Load(paths.New("testdata"), nil)
		require.NoError(t, err)
		require.Len(t, modelsIndex.GetModels(), 2)
	})

	t.Run("custom models folder can be empty", func(t *testing.T) {
		modelsIndex, err := Load(nil, paths.New(t.TempDir()))
		require.NoError(t, err)
		require.Len(t, modelsIndex.GetModels(), 0)
	})

	t.Run("it loads nested custom models correctly", func(t *testing.T) {
		modelsIndex, err := Load(nil, paths.New("testdata/with-nested-models"))
		assert.NoError(t, err)
		assert.NotEmpty(t, modelsIndex)
		assert.Len(t, modelsIndex.GetModels(), 2)

		got := modelsIndex.GetModels()

		assert.Equal(t, f.Must(filepath.Abs("testdata/with-nested-models/nested/nested-model")), got[1].ModelFolderPath.String())
		assert.Equal(t, "my-nested-model-id", got[1].ID)

		assert.Equal(t, f.Must(filepath.Abs("testdata/with-nested-models/another-model")), got[0].ModelFolderPath.String())
		assert.Equal(t, "another-model-id", got[0].ID)
	})

	t.Run("it gets a preloaded model by ID", func(t *testing.T) {
		modelsIndex, err := Load(paths.New("testdata"), paths.New("testdata/models"))
		require.NoError(t, err)
		model, found := modelsIndex.GetModelByID("not-existing-model")
		assert.False(t, found)
		assert.Nil(t, model)

		model, found = modelsIndex.GetModelByID("face-detection")
		require.True(t, found)
		assert.Equal(t, &AIModel{
			ID:                "face-detection",
			Name:              "Lightweight-Face-Detection",
			ModuleDescription: "Face bounding box detection. This model is trained on the WIDER FACE dataset and can detect faces in images.",
			Bricks: []BrickConfig{
				{ID: "arduino:object_detection", ModelConfiguration: map[string]string{"EI_OBJ_DETECTION_MODEL": "/models/ootb/ei/lw-face-det.eim"}},
				{ID: "arduino:video_object_detection", ModelConfiguration: map[string]string{"EI_V_OBJ_DETECTION_MODEL": "/models/ootb/ei/video-face-det.eim"}},
			},
			Metadata: map[string]string{
				"source":           "qualcomm-ai-hub",
				"ei-gpu-mode":      "false",
				"source-model-id":  "face-det-lite",
				"source-model-url": "https://aihub.qualcomm.com/models/face_det_lite",
			},
			ModelLabels: []string{"face"},
			Runner:      "brick",
			IsInternal:  true,
		}, model)
	})

	t.Run("it get custom model by id", func(t *testing.T) {
		modelsIndex, err := Load(paths.New("testdata"), paths.New("testdata/models"))
		require.NoError(t, err)

		eimodel, found := modelsIndex.GetModelByID("my-model-id")
		assert.True(t, found)
		assert.NotNil(t, eimodel)

		assert.Equal(t, &AIModel{
			ID:                "my-model-id",
			Name:              "my custom model from edge impulse",
			ModuleDescription: "A small and accurate model for detecting bounding boxes for faces in images.",
			Bricks:            []BrickConfig{{ID: "object-detection", ModelConfiguration: map[string]string{"AN_ENV_VARIABLE": "/my/env7variable"}}},
			Metadata: map[string]string{
				"a-bool-metadata":   "true",
				"a-int-metadata":    "1",
				"a-string-metadata": "a-string-value",
			},
			ModelFolderPath: paths.New(f.Must(filepath.Abs("testdata/models/my-custom-model"))),
		}, eimodel)
	})

	t.Run("it fails if model-list.yaml does not exist", func(t *testing.T) {
		nonExistentPath := paths.New("nonexistentdir")
		modelsIndex, err := Load(nonExistentPath, nil)
		assert.Error(t, err)
		assert.Nil(t, modelsIndex)
	})

	t.Run("it gets models by a brick", func(t *testing.T) {
		modelsIndex, err := Load(paths.New("testdata"), paths.New("testdata/models"))
		require.NoError(t, err)

		model := modelsIndex.GetModelsByBrick("not-existing-brick")
		assert.Nil(t, model)

		model = modelsIndex.GetModelsByBrick("arduino:object_detection")
		assert.Len(t, model, 1)
		assert.Equal(t, "face-detection", model[0].ID)
	})

	t.Run("it gets models by bricks", func(t *testing.T) {
		modelsIndex, err := Load(paths.New("testdata"), paths.New("testdata/models"))
		require.NoError(t, err)

		models := modelsIndex.GetModelsByBricks([]string{"arduino:non_existing"})
		assert.Len(t, models, 0)
		assert.Nil(t, models)

		models = modelsIndex.GetModelsByBricks([]string{"arduino:video_object_detection"})
		assert.Len(t, models, 2)
		assert.Equal(t, "face-detection", models[0].ID)
		assert.Equal(t, "yolox-object-detection", models[1].ID)

		models = modelsIndex.GetModelsByBricks([]string{"arduino:object_detection", "arduino:video_object_detection"})
		assert.Len(t, models, 2)
		assert.Equal(t, "face-detection", models[0].ID)
		assert.Equal(t, "yolox-object-detection", models[1].ID)
	})
}
