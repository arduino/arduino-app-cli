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
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
)

func TestExportAppZip(t *testing.T) {
	type testCase struct {
		name             string
		appName          string
		files            map[string]string
		nonExistent      bool
		includeData      bool
		wantFiles        []string
		wantMissingFiles []string
		wantErr          bool
		wantFilename     string
	}

	tests := []testCase{
		{
			name:    "Standard app name (include_data=false)",
			appName: "My Test App",
			files: map[string]string{
				"app.yaml":     "content",
				"data/foo.txt": "data content",
			},
			includeData:      false,
			wantErr:          false,
			wantFilename:     "my-test-app.zip",
			wantFiles:        []string{"app.yaml"},
			wantMissingFiles: []string{"data/foo.txt"},
		},
		{
			name:    "Include Data directory (include_data=true)",
			appName: "Data App",
			files: map[string]string{
				"app.yaml":     "content",
				"data/foo.txt": "data content",
			},
			includeData:      true,
			wantErr:          false,
			wantFilename:     "data-app.zip",
			wantFiles:        []string{"app.yaml", "data/foo.txt"},
			wantMissingFiles: []string{},
		},
		{
			name:    "Empty app name uses default",
			appName: "",
			files: map[string]string{
				"app.yaml": "content",
			},
			includeData:  false,
			wantErr:      false,
			wantFilename: "app-export.zip",
			wantFiles:    []string{"app.yaml"},
		},
		{
			name:        "Error on non existent path",
			appName:     "Broken App",
			nonExistent: true,
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			for path, content := range tc.files {
				fullPath := filepath.Join(tmpDir, path)
				require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
				require.NoError(t, os.WriteFile(fullPath, []byte(content), 0600))
			}

			appPath := tmpDir
			if tc.nonExistent {
				appPath = filepath.Join(tmpDir, "not-existing")
			}

			app := ArduinoApp{
				Name:     tc.appName,
				FullPath: paths.New(appPath),
			}
			zipData, filename, err := ExportAppZip(context.Background(), app, tc.includeData)

			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, zipData)
				require.Empty(t, filename)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.wantFilename, filename)
			require.NotEmpty(t, zipData)

			zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
			require.NoError(t, err)

			presentFiles := make(map[string]bool)
			for _, f := range zipReader.File {
				presentFiles[f.Name] = true
			}

			for _, file := range tc.wantFiles {
				require.True(t, presentFiles[file], "File expected in zip but missing: %s", file)
			}

			for _, file := range tc.wantMissingFiles {
				require.False(t, presentFiles[file], "File should NOT be in zip but was found: %s", file)
			}
		})
	}
}
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
			zipData, err := zipAppToBuffer(sourcePath, tc.includeData)

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

func TestValidateZipContent(t *testing.T) {
	tests := []struct {
		name          string
		files         map[string]string
		wantErr       bool
		errorContains string
	}{
		{
			name: "Success - Minimal (app.yaml + python)",
			files: map[string]string{
				"app.yaml":       "",
				"python/main.py": "print('hello')",
			},
			wantErr: false,
		},
		{
			name: "Success - Full with Sketch",
			files: map[string]string{
				"app.yaml":           "",
				"python/main.py":     "",
				"sketch/sketch.ino":  "",
				"sketch/sketch.yaml": "",
			},
			wantErr: false,
		},
		{
			name: "Error - Missing app.yaml",
			files: map[string]string{
				"python/main.py": "",
			},
			wantErr:       true,
			errorContains: "missing app.yaml",
		},
		{
			name: "Error - Missing python/main.py",
			files: map[string]string{
				"app.yaml": "",
			},
			wantErr:       true,
			errorContains: "missing python/main.py",
		},
		{
			name: "Error - Sketch folder present but missing .ino",
			files: map[string]string{
				"app.yaml":           "",
				"python/main.py":     "",
				"sketch/sketch.yaml": "",
			},
			wantErr:       true,
			errorContains: "missing .ino file",
		},
		{
			name: "Error - Sketch folder present but missing .yaml",
			files: map[string]string{
				"app.yaml":          "",
				"python/main.py":    "",
				"sketch/sketch.ino": "",
			},
			wantErr:       true,
			errorContains: "missing .yaml file",
		},
		{
			name: "Success - Extra files are allowed",
			files: map[string]string{
				"app.yaml":       "",
				"python/main.py": "",
				"README.md":      "",
				"data/image.png": "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := createMockZip(t, tt.files)

			err := ValidateAppZipContent(r)

			if (err != nil) != tt.wantErr {
				t.Errorf("validateZipContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errorContains != "" {
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("validateZipContent() error = %v, expected to contain %v", err, tt.errorContains)
				}
			}
		})
	}
}

func createMockZip(t *testing.T, files map[string]string) *zip.Reader {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.Write([]byte(content))
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	return r
}
