// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/gosimple/slug"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

func TestExportAppZip(t *testing.T) {
	bricksIndex, err := bricksindex.Load(platform.GetPlatform(nil), paths.New("testdata", "archive"))
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
			rootFolder := strings.TrimSuffix(tc.wantFilename, ".zip")

			for _, file := range tc.wantFiles {
				expectedPathInZip := path.Join(rootFolder, file)

				_, ok := presentFiles[expectedPathInZip]
				require.True(t, ok, "File expected in zip but missing: %s", expectedPathInZip)
			}

			for _, file := range tc.wantMissingFiles {
				unexpectedPathInZip := path.Join(rootFolder, file)

				_, ok := presentFiles[unexpectedPathInZip]
				require.False(t, ok, "File should NOT be in zip but was found: %s", unexpectedPathInZip)
			}
			appYaml, err := os.ReadFile(filepath.Join("testdata", "archive", "app.redacted.yaml"))
			assert.NoError(t, err)

			zipAppYamlPath := path.Join(rootFolder, "app.yaml")
			assert.Equal(t, string(appYaml), string(presentFiles[zipAppYamlPath]), "Content of app.yaml mismatch")
		})
	}
}

func TestImportAppFromZip(t *testing.T) {
	type testCase struct {
		name            string
		originalZipName string
		zipFiles        map[string]string
		preExisting     bool
		wantErr         bool
		errorContains   string
		expectedFolder  string
	}

	tests := []testCase{
		{
			name:            "Success - Standard App (Flat ZIP)",
			originalZipName: "My App.zip",
			zipFiles: map[string]string{
				"app.yaml":       "name: ignored",
				"python/main.py": "print('hello')",
			},
			expectedFolder: "my-app",
			wantErr:        false,
		},
		{
			name:            "Success - Root Folder Convention",
			originalZipName: "upload.zip",
			zipFiles: map[string]string{
				"root-folder/app.yaml":       "name: ignored",
				"root-folder/python/main.py": "pass",
			},
			expectedFolder: "root-folder",
			wantErr:        false,
		},
		{
			name:            "Success - Conflict Resolution (Suffix)",
			originalZipName: "existing-app.zip",
			zipFiles: map[string]string{
				"app.yaml":       "name: test",
				"python/main.py": "pass",
			},
			preExisting: true,
			wantErr:     false,
		},
		{
			name:            "Success - valid app name starting with dash",
			originalZipName: "test.zip",
			zipFiles: map[string]string{
				"app.yaml":       "name: -invalid-name",
				"python/main.py": "pass",
			},
			wantErr:       false,
			errorContains: "is not valid",
		},
		{
			name:            "Error - Too Deep Structure",
			originalZipName: "test.zip",
			zipFiles: map[string]string{
				"dir1/dir2/app.yaml": "name: test",
			},
			wantErr:       true,
			errorContains: "missing or misplaced app.yaml",
		},
		{
			name:            "Error - Missing python/main.py",
			originalZipName: "valid-name.zip",
			zipFiles: map[string]string{
				"app.yaml": "name: test",
			},
			wantErr:       true,
			errorContains: "main python file missing from app",
		},
		{
			name:            "Error - Zip Slip Attack",
			originalZipName: "evil.zip",
			zipFiles: map[string]string{
				"app.yaml":       "name: hacker",
				"python/main.py": "",
				"../../evil.sh":  "echo pwned",
			},
			wantErr:       true,
			errorContains: "illegal file path",
		},
		{
			name:            "Error - Sketch folder missing .ino",
			originalZipName: "test.zip",
			zipFiles: map[string]string{
				"app.yaml":           "name: test",
				"python/main.py":     "pass",
				"sketch/sketch.yaml": "",
			},
			wantErr:       true,
			errorContains: "both sketch.ino and sketch.yaml are required",
		},
		{
			name:            "Error - Sketch folder missing .yaml",
			originalZipName: "test.zip",
			zipFiles: map[string]string{
				"app.yaml":          "name: test",
				"python/main.py":    "pass",
				"sketch/sketch.ino": "",
			},
			wantErr:       true,
			errorContains: "both sketch.ino and sketch.yaml are required",
		},
		{
			name:            "Error - Nested app missing main.py",
			originalZipName: "test.zip",
			zipFiles: map[string]string{
				"cool-app/app.yaml": "name: test",
			},
			wantErr:       true,
			errorContains: "main python file missing from app",
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
				// create pre-existing app folder to force conflict
				baseName := strings.TrimSuffix(tc.originalZipName, filepath.Ext(tc.originalZipName))
				existsPath := filepath.Join(appsDirPath, slug.Make(baseName))
				require.NoError(t, os.MkdirAll(existsPath, 0755))
			}

			zipPath := filepath.Join(tmpRoot, "temp_import.zip")
			createZipFile(t, zipPath, tc.zipFiles)
			id, err := ImportAppFromZip(cfg, paths.New(zipPath), idProvider, tc.originalZipName)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errorContains != "" {
					require.Contains(t, err.Error(), tc.errorContains)
				}
				require.Empty(t, id)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, id)

				// Verify temp folder cleanup
				files, _ := os.ReadDir(appsDirPath)
				for _, f := range files {
					require.False(t, strings.HasPrefix(f.Name(), ".tmp_"), "Temp folder not cleaned: %s", f.Name())
				}

				if !tc.preExisting && tc.expectedFolder != "" {
					finalPath := cfg.AppsDir().Join(tc.expectedFolder)
					require.True(t, finalPath.Exist(), "App folder should be %s", tc.expectedFolder)
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

func TestFindZipRoot(t *testing.T) {
	WantErrMeessage := "invalid archive structure: missing or misplaced app.yaml. Supported paths: archive.zip/app.yaml or archive.zip/<root_dir>/app.yaml"
	tests := []struct {
		name     string
		files    []string
		wantRoot string
		wantErr  bool
	}{
		{
			name:     "No root folder",
			files:    []string{"app.yaml", "python/main.py"},
			wantRoot: "",
			wantErr:  false,
		},
		{
			name:     "Nested root folder",
			files:    []string{"my-app/app.yaml", "my-app/python/main.py"},
			wantRoot: "my-app",
			wantErr:  false,
		},
		{
			name:     "Deep Nested folder",
			files:    []string{"deep/nested/app.yaml"},
			wantRoot: "deep/nested",
			wantErr:  true,
		},
		{
			name:     "Invalid: Very deep nested folder",
			files:    []string{"deep/nested/folder/app.yaml"},
			wantRoot: "",
			wantErr:  true,
		},
		{
			name:     "Missing app.yaml",
			files:    []string{"data/python/main.py", "README.md"},
			wantRoot: "",
			wantErr:  true,
		},
		{
			name:     "Invalid file name",
			files:    []string{"somethingapp.yaml"},
			wantRoot: "",
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			zipWriter := zip.NewWriter(buf)

			for _, fname := range tc.files {
				_, err := zipWriter.Create(fname)
				require.NoError(t, err)
			}
			require.NoError(t, zipWriter.Close())

			zipReader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
			require.NoError(t, err)

			gotRoot, err := findZipRoot(zipReader)

			if tc.wantErr {
				require.Error(t, err)
				require.Equal(t, WantErrMeessage, err.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantRoot, gotRoot)
			}
		})
	}
}
