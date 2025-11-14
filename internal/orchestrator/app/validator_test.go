package app

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

func TestValidateAppDescriptorBricks(t *testing.T) {
	bricksIndex, err := bricksindex.GenerateBricksIndexFromFile(paths.New("testdata/validator"))
	require.Nil(t, err)
	require.NotNil(t, bricksIndex)

	testCases := []struct {
		name           string
		filename       string
		expectedErrors []error // Now expecting a slice of error messages
	}{
		{
			name:           "valid with all required filled",
			filename:       "all-required-app.yaml",
			expectedErrors: nil,
		},
		{
			name:           "valid with missing bricks",
			filename:       "no-bricks-app.yaml",
			expectedErrors: nil,
		},
		{
			name:           "valid with empty list of bricks",
			filename:       "empty-bricks-app.yaml",
			expectedErrors: nil,
		},
		{
			name:           "valid if required variable is empty string",
			filename:       "empty-required-app.yaml",
			expectedErrors: nil,
		},
		{
			name:     "invalid if required variable is omitted",
			filename: "omitted-required-app.yaml",
			expectedErrors: []error{
				errors.New("variable \"ARDUINO_DEVICE_ID\" is required by brick \"arduino:arduino_cloud\""),
				errors.New("variable \"ARDUINO_SECRET\" is required by brick \"arduino:arduino_cloud\""),
			},
		},
		{
			name:     "invalid if a required variable among two is omitted",
			filename: "omitted-mixed-required-app.yaml",
			expectedErrors: []error{
				errors.New("variable \"ARDUINO_SECRET\" is required by brick \"arduino:arduino_cloud\""),
			},
		},
		{
			name:     "invalid if brick id not found",
			filename: "not-found-brick-app.yaml",
			expectedErrors: []error{
				errors.New("brick \"arduino:not_existing_brick\" not found"),
			},
		},
		{
			name:     "invalid if variable does not exist in the brick",
			filename: "not-found-variable-app.yaml",
			expectedErrors: []error{
				errors.New("variable \"NOT_EXISTING_VARIABLE\" does not exist on brick \"arduino:arduino_cloud\""),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			appDescriptor, err := ParseDescriptorFile(paths.New("testdata/validator/" + tc.filename))
			require.NoError(t, err)

			errs := appDescriptor.ValidateBricks(bricksIndex)
			if tc.expectedErrors == nil {
				require.Nil(t, errs)
				assert.Empty(t, errs, "Expected no validation errors")
			} else {
				require.Len(t, errs, len(tc.expectedErrors), "Expected %d errors but got %d", len(tc.expectedErrors), len(errs))
				require.Exactly(t, tc.expectedErrors, errs)
			}
		})
	}
}
