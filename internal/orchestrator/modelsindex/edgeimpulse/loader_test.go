package edgeimpulse

import (
	"testing"

	paths "github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListEdgeImpulseModels(t *testing.T) {
	t.Parallel()

	ei := New(paths.New("testdata/ei-models-a"))

	models, err := ei.List()
	require.NoError(t, err, "List should not return an error when reading valid testdata")
	require.Len(t, models, 1, "expected exactly one model loaded from testdata")

	expectedModel := ModelDescriptor{
		ProjectId:   152350,
		ImpulseID:   1,
		Name:        "my custom model from edge impulse",
		Description: "A small and accurate model for detecting bounding boxes for faces in images.",
		Category:    "Images",
		Path:        "testdata/ei-models-a/152350/1",
	}
	assert.Equal(t, expectedModel, models[0], "loaded model does not match expected model")
}
