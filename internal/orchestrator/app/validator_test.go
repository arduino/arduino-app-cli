package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

func TestValidateBricksOnAppDescriptor(t *testing.T) {
	bricksIndex, err := bricksindex.GenerateBricksIndexFromFile(paths.New("testdata/validator"))
	require.Nil(t, err)

	testCases := []struct {
		name        string
		filename    string
		expectedErr *string
	}{
		{
			name:        "valid app descriptor with no bricks",
			filename:    "no-bricks-app.yaml",
			expectedErr: nil,
		},
		{
			name:        "valid app descriptor with empty list of bricks",
			filename:    "empty-bricks-app.yaml",
			expectedErr: nil,
		},
		{
			name:        "invalid app descriptor with missing required variable",
			filename:    "missing-required-app.yaml",
			expectedErr: f.Ptr("variable \"ARDUINO_DEVICE_ID\" is required by brick \"arduino:arduino_cloud\""),
		},
		{
			name:        "invalid app descriptor with a missing required variable among two",
			filename:    "mixed-required-app.yaml",
			expectedErr: f.Ptr("variable \"ARDUINO_SECRET\" is required by brick \"arduino:arduino_cloud\""),
		},
		{
			name:        "invalid app descriptor with not found brick id",
			filename:    "not-found-brick-app.yaml",
			expectedErr: f.Ptr("brick \"arduino:not_existing_brick\" not found"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app, err := ParseDescriptorFile(paths.New("testdata/validator/" + tc.filename))
			require.NoError(t, err)

			err = ValidateBricks(app, bricksIndex)

			if tc.expectedErr == nil {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Equal(t, *tc.expectedErr, err.Error())
			}
		})
	}
}
