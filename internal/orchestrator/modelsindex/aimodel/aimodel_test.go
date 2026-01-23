package aimodel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"
)

func TestLoad(t *testing.T) {
	t.Run("it fails if the model path is nil", func(t *testing.T) {
		model, err := Load(nil)
		assert.Error(t, err)
		assert.Empty(t, model)
		assert.Contains(t, err.Error(), "empty model path")
	})

	t.Run("it fails if the model path is empty", func(t *testing.T) {
		model, err := Load(paths.New(""))
		assert.Error(t, err)
		assert.Empty(t, model)
		assert.Contains(t, err.Error(), "empty model path")
	})

	t.Run("it fails if the model path does not exist", func(t *testing.T) {
		_, err := Load(paths.New("testdata/this-folder-does-not-exist"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "model path is not valid")
	})

	t.Run("it loads a model correctly", func(t *testing.T) {
		model, err := Load(paths.New("testdata/my-model"))
		assert.NoError(t, err)
		assert.NotEmpty(t, model)

		assert.Equal(t, ModelDescriptor{
			ID:          "my-model-id",
			Name:        "my custom model name",
			Description: "my description",
			Bricks: []BrickConfig{
				{
					ID: "arduino:a-brick-id",
					ModelConfiguration: map[string]any{
						"MY_ENV_1": "prod",
						"MY_ENV_2": true,
					},
				},
			},
			Metadata: map[string]any{
				"a-string-metadata": "a-string-value",
				"a-bool-metadata":   false,
				"a-int-metadata":    uint64(717280),
			},
		}, model.ModelDescriptor)

		assert.Equal(t, f.Must(filepath.Abs("testdata/my-model")), model.FullPath.String())
	})
}

func TestSave(t *testing.T) {
	t.Run("it writes model.yaml matching golden file", func(t *testing.T) {
		tempDir := t.TempDir()

		model := CustomAiModel{
			FullPath: paths.New(tempDir),
			ModelDescriptor: ModelDescriptor{
				ID:          "my-model-id",
				Name:        "my custom model",
				Description: "test description",
				Bricks: []BrickConfig{
					{
						ID: "arduino:a-brick-id",
						ModelConfiguration: map[string]any{
							"MY_ENV_1": "prod",
							"MY_ENV_2": true,
						},
					},
				},
				Metadata: map[string]any{
					"a-string-metadata": "a-string-value",
					"a-int-metadata":    1,
					"a-bool-metadata":   true,
				},
			},
		}

		err := model.Save()
		require.NoError(t, err)

		descriptorPath := model.GetDescriptorPath()
		require.True(t, descriptorPath.Exist())

		got, err := os.ReadFile(descriptorPath.String())
		require.NoError(t, err)

		goldenPath := paths.New("testdata", "save-model.golden.yaml")
		expected, err := os.ReadFile(goldenPath.String())
		require.NoError(t, err)

		assert.Equal(t, string(expected), string(got))
	})
}
