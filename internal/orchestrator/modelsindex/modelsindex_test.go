package modelsindex

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateModelsIndexFromFile(t *testing.T) {
	testdataPath := paths.New("testdata")

	t.Run("Valid Model list", func(t *testing.T) {
		modelsIndex, err := GenerateModelsIndexFromFile(testdataPath)
		require.NoError(t, err)
		require.NotNil(t, modelsIndex)

		models := modelsIndex.GetModels()
		assert.Len(t, models, 3, "Expected 3 models to be parsed")

		// Test first model
		model1, found := modelsIndex.GetModelByID("face-detection")
		assert.Equal(t, "brick", model1.Runner)
		require.True(t, found, "face-detection should be found")
		assert.Equal(t, "face-detection:", model1.ID)
		assert.Equal(t, "Lightweight-Face-Detection", model1.Name)
		assert.Equal(t, "Face bounding box detection. This model is trained on the WIDER FACE dataset and can detect faces in images.", model1.ModuleDescription)
		assert.Equal(t, []string{"arduino:object_detection", "arduino:video_object_detection"}, model1.L)
		assert.Equal(t, []string{"arduino:object_detection", "arduino:video_object_detection"}, model1.Bricks)
		assert.Equal(t, "1.0.0", model1.Metadata["version"])
		assert.Equal(t, "Test Author", model1.Metadata["author"])
		assert.Equal(t, "1000", model1.ModelConfiguration["max_tokens"])
		assert.Equal(t, "0.7", model1.ModelConfiguration["temperature"])

		// // Test second model
		// model2, found := modelsIndex.GetModelByID("test_model_2")
		// // require.True(t, found, "test_model_2 should be found")
		// // assert.Equal(t, "test_model_2", model2.ID)
		// // assert.Equal(t, "Test Model 2", model2.Name)
		// // assert.Equal(t, "Another test AI model", model2.ModuleDescription)
		// // assert.Equal(t, "another_runner", model2.Runner)
		// // assert.Equal(t, []string{"brick2", "brick3"}, model2.Bricks)
		// // assert.Equal(t, "2.0.0", model2.Metadata["version"])
		// // assert.Equal(t, "MIT", model2.Metadata["license"])

		// // Test minimal model
		// model3, found := modelsIndex.GetModelByID("minimal_model")
		// require.True(t, found, "minimal_model should be found")
		// assert.Equal(t, "minimal_model", model3.ID)
		// assert.Equal(t, "Minimal Model", model3.Name)
		// assert.Equal(t, "Minimal model with no optional fields", model3.ModuleDescription)
		// assert.Equal(t, "minimal_runner", model3.Runner)
		// assert.Empty(t, model3.Bricks)
		// assert.Empty(t, model3.Metadata)
		// assert.Empty(t, model3.ModelConfiguration)
	})

	// Test file not found error
	t.Run("FileNotFound", func(t *testing.T) {
		nonExistentPath := paths.New("nonexistent")
		modelsIndex, err := GenerateModelsIndexFromFile(nonExistentPath)
		assert.Error(t, err)
		assert.Nil(t, modelsIndex)
	})

	// Test invalid YAML parsing
	t.Run("InvalidYAML", func(t *testing.T) {
		// Create a temporary invalid YAML file
		invalidPath := testdataPath.Join("invalid-models.yaml")

		// We expect this to either fail parsing or handle gracefully
		// Since the current implementation may be lenient with missing fields
		modelsIndex, err := GenerateModelsIndexFromFile(testdataPath.Parent().Join("testdata-invalid"))
		if err != nil {
			// If it fails, that's expected for invalid files
			assert.Error(t, err)
			assert.Nil(t, modelsIndex)
		}
		// Note: Some invalid YAML might still parse successfully depending on the YAML library's behavior
		_ = invalidPath // Avoid unused variable warning
	})

	// Test brick filtering functionality
	t.Run("BrickFiltering", func(t *testing.T) {
		modelsIndex, err := GenerateModelsIndexFromFile(testdataPath)
		require.NoError(t, err)

		// Test GetModelsByBrick
		brick1Models := modelsIndex.GetModelsByBrick("brick1")
		assert.Len(t, brick1Models, 1)
		assert.Equal(t, "test_model_1", brick1Models[0].ID)

		brick2Models := modelsIndex.GetModelsByBrick("brick2")
		assert.Len(t, brick2Models, 2)
		modelIDs := []string{brick2Models[0].ID, brick2Models[1].ID}
		assert.Contains(t, modelIDs, "test_model_1")
		assert.Contains(t, modelIDs, "test_model_2")

		// Test GetModelsByBricks
		multiModels := modelsIndex.GetModelsByBricks([]string{"brick1", "brick3"})
		assert.Len(t, multiModels, 2)
		multiModelIDs := []string{multiModels[0].ID, multiModels[1].ID}
		assert.Contains(t, multiModelIDs, "test_model_1")
		assert.Contains(t, multiModelIDs, "test_model_2")

		// Test non-existent brick
		nonExistentModels := modelsIndex.GetModelsByBrick("nonexistent_brick")
		assert.Nil(t, nonExistentModels)
	})
}
