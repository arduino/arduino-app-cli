package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

const validBrickID = "arduino:arduino_cloud"

func setupTestStore(t *testing.T) (*StaticStore, string) {
	cfg, err := config.NewFromEnv()
	require.NoError(t, err)
	baseDir := paths.New("testdata", "assets", cfg.RunnerVersion).String()
	return NewStaticStore(baseDir), baseDir
}

func TestGetBrickReadmeFromID(t *testing.T) {
	store, baseDir := setupTestStore(t)
	namespace, brickName, _ := parseBrickID(validBrickID)
	expectedReadmePath := filepath.Join(baseDir, "docs", namespace, brickName, "README.md")
	expectedContent, err := os.ReadFile(expectedReadmePath)
	require.NoError(t, err, "Error Reading README file: %s", expectedReadmePath)
	require.NotEmpty(t, expectedContent, "ReadME file is empty: %s", expectedReadmePath)

	testCases := []struct {
		name        string
		brickID     string
		wantContent string
		wantErr     bool
		wantErrIs   error
		wantErrMsg  string
	}{
		{
			name:        "Success - file found",
			brickID:     validBrickID,
			wantContent: string(expectedContent),
			wantErr:     false,
		},
		{
			name:        "Failure - file not found",
			brickID:     "namespace:non_existent_brick",
			wantContent: "",
			wantErr:     true,
			wantErrIs:   os.ErrNotExist,
		},
		{
			name:        "Failure - invalid ID",
			brickID:     "invalid-id",
			wantContent: "",
			wantErr:     true,
			wantErrMsg:  "invalid ID",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content, err := store.GetBrickReadmeFromID(tc.brickID)
			if tc.wantErr {
				require.Error(t, err, "should have returned an error")
				if tc.wantErrIs != nil {
					require.ErrorIs(t, err, tc.wantErrIs, "error type mismatch")
				}
				if tc.wantErrMsg != "" {
					require.EqualError(t, err, tc.wantErrMsg, "error message mismatch")
				}
			} else {
				require.NoError(t, err, "should not have returned an error")
			}
			require.Equal(t, tc.wantContent, content, "content mismatch")
		})
	}
}

func TestGetBrickComposeFilePathFromID(t *testing.T) {
	store, baseDir := setupTestStore(t)
	namespace, brickName, _ := parseBrickID(validBrickID)
	expectedPathString := filepath.Join(baseDir, "compose", namespace, brickName, "brick_compose.yaml")

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
			wantPath: expectedPathString,
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
			path, err := store.GetBrickComposeFilePathFromID(tc.brickID)
			if tc.wantErr {
				require.Error(t, err, "function was expected to return an error")
				require.Nil(t, path, "path was expected to be nil")
				require.EqualError(t, err, tc.wantErrMsg, "error message mismatch")
			} else {
				require.NoError(t, err, "function was not expected to return an error")
				require.NotNil(t, path, "path was expected to be not nil")
				require.Equal(t, tc.wantPath, path.String(), "path string mismatch")
			}
		})
	}
}

func TestGetBrickCodeExamplesPathFromID(t *testing.T) {
	store, _ := setupTestStore(t)
	const expectedEntryCount = 2

	testCases := []struct {
		name           string
		brickID        string
		wantNilList    bool
		wantEntryCount int
		wantErr        bool
		wantErrMsg     string
	}{
		{
			name:           "Success - directory found",
			brickID:        validBrickID,
			wantNilList:    false,
			wantEntryCount: expectedEntryCount,
			wantErr:        false,
		},
		{
			name:        "Success - directory not found",
			brickID:     "namespace:non_existent_brick",
			wantNilList: true,
			wantErr:     false,
		},
		{
			name:        "Failure - invalid ID",
			brickID:     "invalid-id",
			wantNilList: true,
			wantErr:     true,
			wantErrMsg:  "invalid ID",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pathList, err := store.GetBrickCodeExamplesPathFromID(tc.brickID)
			if tc.wantErr {
				require.Error(t, err, "should have returned an error")
				require.EqualError(t, err, tc.wantErrMsg, "error message mismatch")
			} else {
				require.NoError(t, err, "should not have returned an error")
			}
			if tc.wantNilList {
				require.Nil(t, pathList, "pathList should be nil")
			} else {
				require.NotNil(t, pathList, "pathList should not be nil")
			}
			require.Equal(t, tc.wantEntryCount, len(pathList), "entry count mismatch")
		})
	}
}
