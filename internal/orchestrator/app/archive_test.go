// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package app

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestZipAppToBuffer(t *testing.T) {
	type testCase struct {
		name        string
		files       map[string]string
		nonExistent bool
		includeData bool
		wantErr     bool
		wantInZip   []string
		wantMissing []string
	}

	tests := []testCase{
		{
			name: "Standard happy path",
			files: map[string]string{
				"app.yaml":        "content file",
				"assets/icon.png": "image-data",
			},
			includeData: false,
			wantErr:     false,
			wantInZip:   []string{"app.yaml", "assets/icon.png"},
			wantMissing: []string{},
		},
		{
			name: "Exclude 'data' directory (includeData=false)",
			files: map[string]string{
				"app.yaml":       "content",
				"data/file.txt":  "should be ignored",
				"data/image.png": "should be ignored",
			},
			includeData: false,
			wantErr:     false,
			wantInZip:   []string{"app.yaml"},
			wantMissing: []string{"data/file.txt", "data/image.png"},
		},
		{
			name: "Include 'data' directory (includeData=true)",
			files: map[string]string{
				"app.yaml":      "content",
				"data/file.txt": "should be included",
			},
			includeData: true,
			wantErr:     false,
			wantInZip:   []string{"app.yaml", "data/file.txt"},
			wantMissing: []string{},
		},
		{
			name: "Ignore .cache folder at root",
			files: map[string]string{
				"app.yaml":          "content",
				".cache/temp_file":  "junk",
				".cache/sub/folder": "junk",
			},
			includeData: false,
			wantErr:     false,
			wantInZip:   []string{"app.yaml"},
			wantMissing: []string{".cache/temp_file", ".cache/sub/folder"},
		},
		{
			name: "Include hidden files not in .cache",
			files: map[string]string{
				".env":           "SECRET=123",
				"assets/.hidden": "hidden-asset",
			},
			includeData: false,
			wantErr:     false,
			wantInZip:   []string{".env", "assets/.hidden"},
			wantMissing: []string{},
		},
		{
			name: "Ignore nested directories inside .cache",
			files: map[string]string{
				"app.js":              "code",
				".cache/v1/data.json": "cache-data",
			},
			includeData: false,
			wantErr:     false,
			wantInZip:   []string{"app.js"},
			wantMissing: []string{".cache/v1/data.json"},
		},
		{
			name:        "Error on non-existent path",
			files:       map[string]string{},
			nonExistent: true,
			wantErr:     true,
			wantInZip:   nil,
			wantMissing: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			for path, content := range tc.files {
				fullPath := filepath.Join(tmpDir, path)
				err := os.MkdirAll(filepath.Dir(fullPath), 0755)
				require.NoError(t, err)
				err = os.WriteFile(fullPath, []byte(content), 0600)
				require.NoError(t, err)
			}

			sourcePath := tmpDir
			if tc.nonExistent {
				sourcePath = filepath.Join(tmpDir, "not existing path")
			}
			zipData, err := ZipAppToBuffer(sourcePath, tc.includeData)

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, zipData)

			zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
			require.NoError(t, err)

			foundFiles := make(map[string]bool)
			for _, f := range zipReader.File {
				require.False(t, strings.Contains(f.Name, "\\"), "not valid Path separator in %s", f.Name)
				if !f.FileInfo().IsDir() {
					foundFiles[f.Name] = true
				}
			}

			for _, file := range tc.wantInZip {
				require.True(t, foundFiles[file], "Missing file into the zip: %s", file)
			}

			for _, file := range tc.wantMissing {
				require.False(t, foundFiles[file], "present file that should be ignored: %s", file)
			}
		})
	}
}
