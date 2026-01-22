package aimodel

import (
	"os"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
)

func TestParseModelDescription(t *testing.T) {
	modelDescriptor := `
id: "my-model-id"
name: "my custom model from edge impulse"
description: "A small and accurate model for detecting bounding boxes for faces in images."
bricks:
  - "object-detection"
`

	modelYaml := paths.New(t.TempDir(), "model.yaml")

	err := os.WriteFile(modelYaml.String(), []byte(modelDescriptor), 0600)
	require.NoError(t, err)

	descr, err := ParseModelDescriptorFile(modelYaml)
	require.NoError(t, err)

	require.Equal(t, ModelDescriptor{
		ID:          "my-model-id",
		Name:        "my custom model from edge impulse",
		Description: "A small and accurate model for detecting bounding boxes for faces in images.",
		Bricks:      []string{"object-detection"},
	}, descr)

}
