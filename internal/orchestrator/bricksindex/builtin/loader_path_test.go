package builtin

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"
)

var validBrickID = "arduino:arduino_cloud"

func TestGetBrickComposeFilePathFromID(t *testing.T) {
	index := f.Must(Load(paths.New("testdata", "assets", "0.4.8")))

	testCases := []struct {
		name       string
		brickID    string
		wantPath   string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:     "Success - valid ID",
			brickID:  validBrickID,
			wantPath: "testdata/assets/0.4.8/compose/arduino/arduino_cloud/brick_compose.yaml",
			wantErr:  false,
		},
		{
			name:       "Failure - invalid ID",
			brickID:    "invalid ID",
			wantPath:   "",
			wantErr:    true,
			wantErrMsg: "invalid ID",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path, found := index.GetComposePath(tc.brickID)
			if tc.wantErr {
				require.False(t, found, "function was expected to return false")
				require.Nil(t, path, "path was expected to be nil")
			} else {
				require.True(t, found, "function was expected to return true")
				require.NotNil(t, path, "path was expected to be not nil")
			}
		})
	}
}

func TestGetBrickCodeExamplesPathFromID(t *testing.T) {
	index := f.Must(Load(paths.New("testdata", "assets", "0.4.8")))

	testCases := []struct {
		name           string
		brickID        string
		wantEntryCount int
		wantErr        string
	}{
		{
			name:           "Success - directory found",
			brickID:        validBrickID,
			wantEntryCount: 2,
			wantErr:        "",
		},
		{
			name:           "Success - directory not found",
			brickID:        "namespace:non_existent_brick",
			wantEntryCount: 0,
			wantErr:        "",
		},
		{
			name:           "Failure - invalid ID",
			brickID:        "invalid-id",
			wantEntryCount: 0,
			wantErr:        "invalid ID",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pathList, err := index.GetCodeExamplesPath(tc.brickID)
			if tc.wantErr != "" {
				require.Error(t, err, "should have returned an error")
				require.EqualError(t, err, tc.wantErr, "error message mismatch")
			} else {
				require.NoError(t, err, "should not have returned an error")
			}
			if tc.wantEntryCount == 0 {
				require.Nil(t, pathList, "pathList should be nil")
			} else {
				require.NotNil(t, pathList, "pathList should not be nil")
			}
			require.Equal(t, tc.wantEntryCount, len(pathList), "entry count mismatch")
		})
	}
}
