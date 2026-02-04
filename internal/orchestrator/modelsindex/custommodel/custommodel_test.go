package custommodel

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"
)

func TestLoad(t *testing.T) {
	t.Run("it fails if the model folder path is nil", func(t *testing.T) {
		model, err := Load(nil)
		assert.Error(t, err)
		assert.Empty(t, model)
		assert.Contains(t, err.Error(), "empty model folder path")
	})

	t.Run("it fails if the model folder path is empty", func(t *testing.T) {
		model, err := Load(paths.New(""))
		assert.Error(t, err)
		assert.Empty(t, model)
		assert.Contains(t, err.Error(), "empty model folder path")
	})

	t.Run("it fails if the model folder path does not exist", func(t *testing.T) {
		_, err := Load(paths.New("testdata/this-folder-does-not-exist"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "model folder path is not valid")
	})

	t.Run("it fails if the model descriptor does not exist", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Load(paths.New(dir))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "descriptor model.yaml file missing from app")
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
					ModelConfiguration: map[string]string{
						"MY_ENV_1": "prod",
						"MY_ENV_2": "true",
					},
				},
			},
			Metadata: map[string]string{
				"a-string-metadata": "a-string-value",
				"a-bool-metadata":   "false",
				"a-int-metadata":    "717280",
			},
		}, model.ModelDescriptor)

		assert.Equal(t, f.Must(filepath.Abs("testdata/my-model")), model.FullPath.String())
	})
}

func TestStore(t *testing.T) {

	t.Run("it writes descriptor and model file when reader provided", func(t *testing.T) {
		tempDir := t.TempDir()
		modelDir := paths.New(tempDir).Join("test-model-with-blob")

		descr := ModelDescriptor{
			ID:          "test-id-blob",
			Name:        "test model with blob",
			Description: "test description",
		}

		blobContent := []byte("this is model blob content")
		blobReader := io.NopCloser(bytes.NewReader(blobContent))

		m, err := Store(modelDir, descr, blobReader, "model.blob")
		require.NoError(t, err)

		assert.Equal(t, descr, m.ModelDescriptor)
		assert.Equal(t, modelDir, m.FullPath)

		descriptorPath := modelDir.Join("model.yaml")
		require.True(t, descriptorPath.Exist())

		blobPath := modelDir.Join("model.blob")
		require.True(t, blobPath.Exist())

		gotBlob, err := os.ReadFile(blobPath.String())
		require.NoError(t, err)
		assert.Equal(t, blobContent, gotBlob)
	})

	t.Run("it fails when model reader provided without filename", func(t *testing.T) {
		tempDir := t.TempDir()
		modelDir := paths.New(tempDir).Join("test-model-no-filename")

		descr := ModelDescriptor{
			ID:   "test-id",
			Name: "test",
		}

		blobReader := io.NopCloser(bytes.NewReader([]byte("content")))

		_, err := Store(modelDir, descr, blobReader, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "model filename must be provided")
	})

	t.Run("it writes large model file successfully", func(t *testing.T) {
		tempDir := t.TempDir()
		modelDir := paths.New(tempDir).Join("test-model-large")

		descr := ModelDescriptor{
			ID:   "test-large",
			Name: "large model",
		}

		// Create 1MB blob
		largeBlob := make([]byte, 1024*1024)
		for i := range largeBlob {
			largeBlob[i] = byte(i % 256)
		}
		blobReader := io.NopCloser(bytes.NewReader(largeBlob))

		_, err := Store(modelDir, descr, blobReader, "large-model.blob")
		require.NoError(t, err)

		blobPath := modelDir.Join("large-model.blob")
		require.True(t, blobPath.Exist())

		gotBlob, err := os.ReadFile(blobPath.String())
		require.NoError(t, err)
		assert.Equal(t, len(largeBlob), len(gotBlob))
		assert.Equal(t, largeBlob, gotBlob)
	})

	t.Run("it stores model file via HTTP response body", func(t *testing.T) {
		tempDir := t.TempDir()
		modelDir := paths.New(tempDir).Join("test-model-http")

		descr := ModelDescriptor{
			ID:   "test-http",
			Name: "http model",
		}

		payload := []byte("http served model content")
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(payload)
		}))
		defer ts.Close()

		resp, err := http.Get(ts.URL)
		require.NoError(t, err)
		defer resp.Body.Close()

		_, err = Store(modelDir, descr, resp.Body, "model-http.blob")
		require.NoError(t, err)

		blobPath := modelDir.Join("model-http.blob")
		require.True(t, blobPath.Exist())

		got, err := os.ReadFile(blobPath.String())
		require.NoError(t, err)
		assert.Equal(t, payload, got)
	})
}
