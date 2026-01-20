package modelsindex

import (
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
