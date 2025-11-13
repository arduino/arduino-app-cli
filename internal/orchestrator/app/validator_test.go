package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

func TestValidateAppDescriptor(t *testing.T) {
	bricksIndex, err := bricksindex.GenerateBricksIndexFromFile(paths.New("testdata/validator"))
	require.Nil(t, err)

	t.Run("valid app descriptor with no bricks", func(t *testing.T) {
		app, err := ParseDescriptorFile(paths.New("testdata/validator/no-bricks-app.yaml"))
		require.NoError(t, err)
		err = ValidateAppDescriptor(app, bricksIndex)
		assert.NoError(t, err)
	})

	t.Run("valid app descriptor with empty list of bricks", func(t *testing.T) {
		app, err := ParseDescriptorFile(paths.New("testdata/validator/empty-bricks-app.yaml"))
		require.NoError(t, err)
		err = ValidateAppDescriptor(app, bricksIndex)
		assert.NoError(t, err)
	})

	t.Run("invalid app descriptor with missing required variable", func(t *testing.T) {
		app, err := ParseDescriptorFile(paths.New("testdata/validator/missing-required-app.yaml"))
		require.NoError(t, err)
		err = ValidateAppDescriptor(app, bricksIndex)
		assert.Equal(t, "variable \"ARDUINO_DEVICE_ID\" is required by brick \"arduino:arduino_cloud\"", err.Error())
	})

	t.Run("invalid app descriptor with a missing required variable among two", func(t *testing.T) {
		app, err := ParseDescriptorFile(paths.New("testdata/validator/mixed-required-app.yaml"))
		require.NoError(t, err)
		err = ValidateAppDescriptor(app, bricksIndex)
		assert.Equal(t, "variable \"ARDUINO_DEVICE_ID\" is required by brick \"arduino:arduino_cloud\"", err.Error())
	})

	t.Run("invalid app descriptor with not found brick id", func(t *testing.T) {
		app, err := ParseDescriptorFile(paths.New("testdata/validator/not-found-brick-app.yaml"))
		require.NoError(t, err)
		err = ValidateAppDescriptor(app, bricksIndex)
		assert.Equal(t, "brick \"arduino:not_existing_brick\" not found", err.Error())
	})

}
