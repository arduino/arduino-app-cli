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
func TestValidateAppZipContent(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]string
		rootPrefix  string
		wantErr     bool
		errContains string
	}{
		{
			name: "Valid standard app (Flat Root)",
			files: map[string]string{
				"app.yaml":       "",
				"python/main.py": "",
			},
			rootPrefix: "",
			wantErr:    false,
		},
		{

			name: "Valid nested app (With Root Folder)",
			files: map[string]string{
				"my-app/app.yaml":       "",
				"my-app/python/main.py": "",
			},
			rootPrefix: "my-app",
			wantErr:    false,
		},
		{
			name: "Valid app with yaml variant (.yml)",
			files: map[string]string{
				"app.yml":        "",
				"python/main.py": "",
			},
			rootPrefix: "",
			wantErr:    false,
		},
		{
			name: "Valid app with full sketch folder",
			files: map[string]string{
				"app.yaml":           "",
				"python/main.py":     "",
				"sketch/sketch.ino":  "",
				"sketch/sketch.yaml": "",
			},
			rootPrefix: "",
			wantErr:    false,
		},
		{
			name: "Valid Windows paths (Backslash handling)",
			files: map[string]string{
				"app.yaml":            "",
				"python/main.py":      "",
				"sketch\\sketch.ino":  "",
				"sketch\\sketch.yaml": "",
			},
			rootPrefix: "",
			wantErr:    false,
		},
		{
			name: "Ignore unrelated folders with similar prefix",
			files: map[string]string{
				"app.yaml":               "",
				"python/main.py":         "",
				"sketch_backup/main.cpp": "",
			},
			rootPrefix: "",
			wantErr:    false,
		},
		{
			name: "Missing app.yaml",
			files: map[string]string{
				"python/main.py": "",
			},
			rootPrefix:  "",
			wantErr:     true,
			errContains: "missing app.yaml",
		},
		{
			name: "Missing python/main.py",
			files: map[string]string{
				"app.yaml": "",
			},
			rootPrefix:  "",
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
			rootPrefix:  "",
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
			rootPrefix:  "",
			wantErr:     true,
			errContains: "missing .yaml file",
		},
		{
			name: "Nested App missing main.py (Check relative path logic)",
			files: map[string]string{
				"cool-app/app.yaml": "",
			},
			rootPrefix:  "cool-app",
			wantErr:     true,
			errContains: "missing python/main.py",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := createMockZip(t, tt.files)

			gotErr := validateAppZipContent(r, tt.rootPrefix)

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
			errorContains: "missing python/main.py",
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
			name:     "No root folder (with .yml)",
			files:    []string{"app.yml", "python/main.py"},
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
			files:    []string{"deep/nested/app.yml"},
			wantRoot: "deep/nested",
			wantErr:  true,
		},
		{
			name:     "Invalid: Very deep nested folder",
			files:    []string{"deep/nested/folder/app.yml"},
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
