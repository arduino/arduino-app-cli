package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

func TestValidateAppDescriptorBricks(t *testing.T) {
	bricksIndex, err := bricksindex.GenerateBricksIndexFromFile(paths.New("testdata/validator"))
	require.Nil(t, err)
	require.NotNil(t, bricksIndex)

	testCases := []struct {
		name        string
		filename    string
		expectedErr *string
	}{
		{
			name:        "valid with missing bricks",
			filename:    "no-bricks-app.yaml",
			expectedErr: nil,
		},
		{
			name:        "valid with empty list of bricks",
			filename:    "empty-bricks-app.yaml",
			expectedErr: nil,
		},
		{
			name:        "valid if required variable is empty string",
			filename:    "empty-required-app.yaml",
			expectedErr: nil,
		},
		{
			name:        "invalid if required variable is omitted",
			filename:    "omitted-required-app.yaml",
			expectedErr: f.Ptr("variable \"ARDUINO_DEVICE_ID\" is required by brick \"arduino:arduino_cloud\""),
		},
		{
			name:        "invalid if a required variable among two is omitted",
			filename:    "omitted-mixed-required-app.yaml",
			expectedErr: f.Ptr("variable \"ARDUINO_SECRET\" is required by brick \"arduino:arduino_cloud\""),
		},
		{
			name:        "invalid if brick id not found",
			filename:    "not-found-brick-app.yaml",
			expectedErr: f.Ptr("brick \"arduino:not_existing_brick\" not found"),
		},
		{
			name:        "invalid variable does not exist in the brick",
			filename:    "not-found-variable-app.yaml",
			expectedErr: f.Ptr("variable \"NOT_EXISTING_VARIABLE\" does not exist on brick \"arduino:arduino_cloud\""),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			appDescriptor, err := ParseDescriptorFile(paths.New("testdata/validator/" + tc.filename))
			require.NoError(t, err)

			err = appDescriptor.ValidateBricks(bricksIndex)
			if tc.expectedErr == nil {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Equal(t, *tc.expectedErr, err.Error())
			}
		})
	}
}
