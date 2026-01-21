package aimodel

import (
	"path/filepath"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
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

		assert.Equal(t, f.Must(filepath.Abs("testdata/my-model")), model.FullPath.String())
	})
}

func TestSave(t *testing.T) {
	t.Run("it writes model.yaml in empty dir", func(t *testing.T) {
		tempDir := t.TempDir()

		model := CustomAiModel{
			FullPath: paths.New(tempDir),
			ModelDescriptor: ModelDescriptor{
				Name:        "my custom model",
				Description: "test description",
				Bricks:      []string{"object-detection"},
			},
		}

		err := model.Save()
		assert.NoError(t, err)

		descriptorPath := model.GetDescriptorPath()
		assert.True(t, descriptorPath.Exist())

		loaded, err := ParseModelDescriptorFile(descriptorPath)
		assert.NoError(t, err)
		assert.Equal(t, model.ModelDescriptor.Name, loaded.Name)
		assert.Equal(t, model.ModelDescriptor.Description, loaded.Description)
		assert.Equal(t, model.ModelDescriptor.Bricks, loaded.Bricks)
	})
}
