// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package store

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
)

const validBrickID = "arduino:arduino_cloud"

func TestGetBrickReadmeFromID(t *testing.T) {
	store := NewStaticStore(paths.New("testdata", "assets", "0.4.8").String())

	testCases := []struct {
		name        string
		brickID     string
		wantContent string
		wantErr     bool
		wantErrMsg  string
	}{
		{
			name:        "Success - file found",
			brickID:     validBrickID,
			wantContent: "## Readme test file",
			wantErr:     false,
		},
		{
			name:        "Failure - file not found",
			brickID:     "namespace:non_existent_brick",
			wantContent: "",
			wantErr:     true,
			wantErrMsg:  "open testdata/assets/0.4.8/docs/namespace/non_existent_brick/README.md: no such file or directory",
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
	store := NewStaticStore(paths.New("testdata", "assets", "0.4.8").String())

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
	store := NewStaticStore(paths.New("testdata", "assets", "0.4.8").String())

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
			pathList, err := store.GetBrickCodeExamplesPathFromID(tc.brickID)
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
