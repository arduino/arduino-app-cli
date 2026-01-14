package custommodels

import (
	"testing"

	paths "github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListEdgeImpulseModels(t *testing.T) {
	t.Parallel()

	models, err := List(paths.New("testdata/ei-models-a"))
	require.NoError(t, err, "List should not return an error when reading valid testdata")
	require.Len(t, models, 1, "expected exactly one model loaded from testdata")

	expectedModel := EdgeImpulseModel{
		ProjectId:   152350,
		ImpulseID:   1,
		Name:        "my custom model from edge impulse",
		Description: "A small and accurate model for detecting bounding boxes for faces in images.",
		Category:    "Images",
		Path:        "testdata/ei-models-a/152350/1",
	}
	assert.Equal(t, expectedModel, models[0], "loaded model does not match expected model")
}

func TestGetModelsByBrick(t *testing.T) {
	t.Parallel()

	models, err := GetModelsByBrick(paths.New("testdata/ei-models-a"), "non-existing-brick")
	require.NoError(t, err, "List should not return an error when reading valid testdata")
	require.Len(t, models, 0, "expected no models loaded for non-existing brick")

	models, err = GetModelsByBrick(paths.New("testdata/ei-models-a"), "object-detection")
	require.NoError(t, err, "List should not return an error when reading valid testdata")
	require.Len(t, models, 1, "expected exactly one model loaded from testdata")

	expectedModel := EdgeImpulseModel{
		ProjectId:   152350,
		ImpulseID:   1,
		Name:        "my custom model from edge impulse",
		Description: "A small and accurate model for detecting bounding boxes for faces in images.",
		Category:    "Images",
		Path:        "testdata/ei-models-a/152350/1",
	}
	assert.Equal(t, expectedModel, models[0], "loaded model does not match expected model")
}
