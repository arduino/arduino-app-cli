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

package orchestrator

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func TestExportAppZip(t *testing.T) {
	bricksIndex, err := bricksindex.Load(paths.New("testdata", "archive"))
	require.NoError(t, err)

	type testCase struct {
		name             string
		appName          string
		files            []string
		nonExistent      bool
		includeData      bool
		wantFiles        []string
		wantMissingFiles []string
		wantErr          bool
		wantFilename     string
	}

	tests := []testCase{
		{
			name:             "Standard app name (include_data=false)",
			appName:          "My Test App",
			files:            []string{"app.yaml", "data/foo.txt"},
			includeData:      false,
			wantErr:          false,
			wantFilename:     "my-test-app.zip",
			wantFiles:        []string{"app.yaml"},
			wantMissingFiles: []string{"data/foo.txt"},
		},
		{
			name:             "Include Data directory (include_data=true)",
			appName:          "Data App",
			files:            []string{"app.yaml", "data/foo.txt"},
			includeData:      true,
			wantErr:          false,
			wantFilename:     "data-app.zip",
			wantFiles:        []string{"app.yaml", "data/foo.txt"},
			wantMissingFiles: []string{},
		},
		{
			name:         "Empty app name uses default",
			appName:      "",
			files:        []string{"app.yaml", "data/foo.txt"},
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

			writeFiles(t, tmpDir, tc.files)

			appPath := tmpDir
			if tc.nonExistent {
				appPath = filepath.Join(tmpDir, "not-existing")
			}

			app := app.ArduinoApp{
				Name:     tc.appName,
				FullPath: paths.New(appPath),
			}
			zipData, filename, err := ExportAppZip(t.Context(), bricksIndex, app, tc.includeData)

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

			presentFiles := make(map[string][]byte)
			for _, f := range zipReader.File {
				r, err := f.Open()
				assert.NoError(t, err)
				presentFiles[f.Name], err = io.ReadAll(r)
				assert.NoError(t, err)
				r.Close()
			}

			for _, file := range tc.wantFiles {
				_, ok := presentFiles[file]
				require.True(t, ok, "File expected in zip but missing: %s", file)
			}

			for _, file := range tc.wantMissingFiles {
				_, ok := presentFiles[file]
				require.False(t, ok, "File should NOT be in zip but was found: %s", file)
			}

			appYaml, err := os.ReadFile(filepath.Join("testdata", "archive", "app.redacted.yaml"))
			assert.NoError(t, err)
			assert.Equal(t, string(appYaml), string(presentFiles["app.yaml"]))

		})
	}
}

func TestValidateAppZipContent(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]string
		wantErr     bool
		errContains string
	}{
		{
			name: "Valid standard app",
			files: map[string]string{
				"app.yaml":       "",
				"python/main.py": "",
			},
			wantErr: false,
		},
		{
			name: "Valid app with yaml variant (.yml)",
			files: map[string]string{
				"app.yml":        "",
				"python/main.py": "",
			},
			wantErr: false,
		},
		{
			name: "Valid app with full sketch folder",
			files: map[string]string{
				"app.yaml":           "",
				"python/main.py":     "",
				"sketch/sketch.ino":  "",
				"sketch/sketch.yaml": "",
			},
			wantErr: false,
		},
		{
			name: "Valid Windows paths (Backslash handling)",
			files: map[string]string{
				"app.yaml":            "",
				"python/main.py":      "",
				"sketch\\sketch.ino":  "",
				"sketch\\sketch.yaml": "",
			},
			wantErr: false,
		},
		{
			name: "Ignore unrelated folders with similar prefix",
			files: map[string]string{
				"app.yaml":               "",
				"python/main.py":         "",
				"sketch_backup/main.cpp": "",
			},
			wantErr: false,
		},

		{
			name: "Missing app.yaml",
			files: map[string]string{
				"python/main.py": "",
			},
			wantErr:     true,
			errContains: "missing app.yaml",
		},
		{
			name: "Missing python/main.py",
			files: map[string]string{
				"app.yaml": "",
			},
			wantErr:     true,
			errContains: "missing python/main.py",
		},
		{
			name: "Sketch folder present but missing .ino",
			files: map[string]string{
				"app.yaml":           "",
				"python/main.py":     "",
				"sketch/readme.txt":  "",
				"sketch/sketch.yaml": "",
			},
			wantErr:     true,
			errContains: "missing .ino file",
		},
		{
			name: "Sketch folder present but missing .yaml",
			files: map[string]string{
				"app.yaml":          "",
				"python/main.py":    "",
				"sketch/sketch.ino": "",
			},
			wantErr:     true,
			errContains: "missing .yaml file",
		},
		{
			name: "Sketch file exists but in wrong folder",
			files: map[string]string{
				"app.yaml":              "",
				"python/main.py":        "",
				"sketch/lib/sketch.ino": "",
				"sketch/sketch.yaml":    "",
			},
			wantErr:     true,
			errContains: "missing .ino file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := createMockZip(t, tt.files)
			gotErr := validateAppZipContent(r)
			if tt.wantErr {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tt.errContains, "Error message mismatch")
			} else {
				require.NoError(t, gotErr, "Expected success but got an error")
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

func TestImportAppFromZip(t *testing.T) {
	type testCase struct {
		name          string
		folderName    string
		zipFiles      map[string]string
		preExisting   bool
		wantErr       bool
		expectedErr   error
		errorContains string
	}

	tests := []testCase{
		{
			name:       "Success - Standard App",
			folderName: "test-app",
			zipFiles: map[string]string{
				"app.yaml":       "name: Test App",
				"python/main.py": "print('hello')",
			},
			wantErr: false,
		},
		{
			name:       "Success - App with Sketch",
			folderName: "app",
			zipFiles: map[string]string{
				"app.yaml":           "name: app",
				"python/main.py":     "pass",
				"sketch/sketch.ino":  "void setup() {}",
				"sketch/sketch.yaml": "board: unoQ",
			},
			wantErr: false,
		},
		{
			name:       "Success - Ignores junk files",
			folderName: "test",
			zipFiles: map[string]string{
				"app.yaml":       "name: test",
				"python/main.py": "print('hello')",
				"junk/._junk":    "garbage",
			},
			wantErr: false,
		},
		{
			name:       "Error - Empty App Name in YAML",
			folderName: "",
			zipFiles: map[string]string{
				"app.yaml":       "name: \"   \"",
				"python/main.py": "print('h')",
			},
			wantErr:       true,
			expectedErr:   ErrBadRequest,
			errorContains: "app name is missing",
		},
		{
			name:       "Error - App Already Exists",
			folderName: "existing-app",
			zipFiles: map[string]string{
				"app.yaml":       "name: Existing App",
				"python/main.py": "print('hello')",
			},
			preExisting: true,
			wantErr:     true,
			expectedErr: ErrAppAlreadyExists,
		},
		{
			name:       "Error - Missing app.yaml",
			folderName: "no-yaml",
			zipFiles: map[string]string{
				"python/main.py": "print('hello')",
			},
			wantErr:       true,
			expectedErr:   ErrBadRequest,
			errorContains: "missing app.yaml",
		},
		{
			name:       "Error - Missing python/main.py",
			folderName: "test",
			zipFiles: map[string]string{
				"app.yaml": "name: test",
			},
			wantErr:       true,
			expectedErr:   ErrBadRequest,
			errorContains: "missing python/main.py",
		},
		{
			name:       "Error - Sketch missing .ino",
			folderName: "broken-sketch",
			zipFiles: map[string]string{
				"app.yaml":           "name: Broken Sketch",
				"python/main.py":     "",
				"sketch/sketch.yaml": "",
			},
			wantErr:       true,
			expectedErr:   ErrBadRequest,
			errorContains: "missing .ino file",
		},
		{
			name:       "Error - Zip Slip Attack",
			folderName: "hacker-app",
			zipFiles: map[string]string{
				"app.yaml":       "name: hacker",
				"python/main.py": "",
				"../../evil.sh":  "echo pwned",
			},
			wantErr:       true,
			errorContains: "illegal file path",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpRoot := t.TempDir()
			appsDirPath := filepath.Join(tmpRoot, "ArduinoApps")

			t.Setenv("ARDUINO_APP_CLI__APPS_DIR", appsDirPath)
			t.Setenv("ARDUINO_APP_CLI__DATA_DIR", filepath.Join(tmpRoot, "Data"))

			cfg, err := config.NewFromEnv()
			require.NoError(t, err)

			idProvider := app.NewAppIDProvider(cfg)

			if tc.preExisting {
				existsPath := filepath.Join(appsDirPath, tc.folderName)
				require.NoError(t, os.MkdirAll(existsPath, 0755))
			}

			zipPath := filepath.Join(tmpRoot, "import.zip")
			createZipFile(t, zipPath, tc.zipFiles)

			id, err := ImportAppFromZip(cfg, paths.New(zipPath), idProvider)

			if tc.wantErr {
				require.Error(t, err)

				if tc.expectedErr != nil {
					require.Truef(t, errors.Is(err, tc.expectedErr), "want error %v, got %v", tc.expectedErr, err)
				}

				if tc.errorContains != "" {
					require.Contains(t, err.Error(), tc.errorContains)
				}

				require.Empty(t, id)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, id)

				finalPath := cfg.AppsDir().Join(tc.folderName)

				require.True(t, finalPath.Exist(), "App folder should exist at %s", finalPath)
				require.True(t, finalPath.Join("app.yaml").Exist(), "app.yaml missing")
				require.True(t, finalPath.Join("python/main.py").Exist(), "main.py missing")

				files, _ := finalPath.Parent().ReadDir()
				for _, f := range files {
					name := f.Base()
					isTempDir := len(name) > 5 && name[:5] == ".tmp_"
					require.False(t, isTempDir, "Temporary folder not cleaned up: %s", name)
				}
			}
		})
	}
}

func createZipFile(t *testing.T, filename string, files map[string]string) {
	t.Helper()
	f, err := os.Create(filename)
	require.NoError(t, err)
	defer f.Close()

	w := zip.NewWriter(f)

	for name, content := range files {
		f, err := w.Create(name)
		require.NoError(t, err)
		_, err = f.Write([]byte(content))
		require.NoError(t, err)
	}

	require.NoError(t, w.Close())
}

func writeFiles(t *testing.T, tmpPath string, files []string) {
	t.Helper()

	for _, path := range files {
		srcPath := filepath.Join("testdata", "archive", path)
		content, err := os.ReadFile(srcPath)
		require.NoError(t, err)

		dstPath := filepath.Join(tmpPath, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(dstPath), 0755))
		require.NoError(t, os.WriteFile(dstPath, content, 0600))
	}
}
