package modelsindex

import (
	"context"
	"log"
	"os"
	"testing"

	paths "github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListEdgeImpulseModels(t *testing.T) {
	t.Parallel()

	eimodels, err := LoadEdgeImpulseModels(paths.New("testdata/ei-models"))
	require.NoError(t, err)
	require.Len(t, eimodels, 1, "expected exactly one model loaded from testdata")

	expectedModel := AIModel{
		ID:                "my-model-id",
		Source:            "edgeimpulse",
		Runner:            "bricks",
		Name:              "my custom model from edge impulse",
		ModuleDescription: "A small and accurate model for detecting bounding boxes for faces in images.",
		Bricks:            []string{"object-detection"},
		Metadata: map[string]string{
			"project-id": "111111",
			"impulse-id": "1",
		},
		ModelConfiguration: map[string]string{"EI_OBJ_DETECTION_MODEL": "testdata/ei-models/111111/1/my-model.eim"},
	}
	assert.Equal(t, expectedModel, eimodels[0], "loaded model does not match expected model")
}

// TODO should be mocked and add a e2e test with an Arduino organization
func TestInstallEIModel(t *testing.T) {

	customEIModelsDir := paths.New(os.Getenv("ARDUINO_CUSTOM_MODEL_PATH"))
	ApiKey := os.Getenv("ARDUINO_EI_API_KEY")
	ApiUrl := os.Getenv("ARDUINO_EI_API_URL")

	if err := os.MkdirAll(customEIModelsDir.String(), 0o755); err != nil {
		log.Fatalf("failed to create directory %s: %v", customEIModelsDir, err)
	}

	eiClient := NewEIClient(ApiKey, ApiUrl)
	ProjectID := 876194
	ImpulseID := 1

	err := InstallEIModel(context.TODO(), eiClient, *customEIModelsDir, ProjectID, ImpulseID)
	if err != nil {
		log.Fatalf("failed to install EI model: %v", err)
	}

	model, err := SaveEIModel(context.TODO(), eiClient, *customEIModelsDir, ProjectID, ImpulseID)
	if err != nil {
		log.Fatalf("failed to save EI model metadata: %v", err)
	}

	models, err := LoadEdgeImpulseModels(customEIModelsDir)
	if err != nil {
		log.Fatalf("failed to load EI models: %v", err)
	}

	if found, ok := func() (*AIModel, bool) {
		for _, m := range models {
			if m.ID == model.ID {
				return &m, true
			}
		}
		return nil, false
	}(); !ok {
		log.Fatalf("installed model with ID %s not found in loaded models", model.ID)
	} else {
		assert.Equal(t, model.Name, found.Name, "model name does not match")
		assert.Equal(t, model.ModuleDescription, found.ModuleDescription, "model description does not match")
		assert.Equal(t, model.Runner, found.Runner, "model runner does not match")
		assert.Equal(t, model.Bricks, found.Bricks, "model bricks do not match")
		assert.Equal(t, model.Metadata, found.Metadata, "model metadata does not match")
	}

	if err := os.RemoveAll(customEIModelsDir.String()); err != nil {
		log.Fatalf("failed to delete %s: %v", customEIModelsDir, err)
	}
}
