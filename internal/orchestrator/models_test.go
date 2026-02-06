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
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/api/edgeimpulse"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

func TestBuildBrickConfigForEIModel(t *testing.T) {

	brickIndex, err := bricksindex.Load(paths.New("bricksindex/testdata"))
	if err != nil {
		t.Fatalf("failed to load bricks index: %v", err)
	}

	category := edgeimpulse.ProjectCategory("Object detection")
	edgeModelsDir := paths.New("/models/custom-ei/ei-xxxx-yyyy")
	blobModelsDir := paths.New("/models/custom-ei/ei-xxxx-yyyy")

	result, err := buildBrickConfigForEIModel(
		brickIndex,
		&category,
		edgeModelsDir,
		blobModelsDir,
	)

	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, "arduino:object_detection", result[0].ID)
	require.Equal(t, "arduino:video_object_detection", result[1].ID)

	require.Equal(t, map[string]string{
		"CUSTOM_MODEL_PATH":      "/models/custom-ei/ei-xxxx-yyyy",
		"EI_OBJ_DETECTION_MODEL": "/models/custom-ei/ei-xxxx-yyyy",
	}, result[0].ModelConfiguration)
	require.Equal(t, map[string]string{
		"CUSTOM_MODEL_PATH":      "/models/custom-ei/ei-xxxx-yyyy",
		"EI_OBJ_DETECTION_MODEL": "/models/custom-ei/ei-xxxx-yyyy",
	}, result[1].ModelConfiguration)
}
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
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
)

func createFileWithSize(t *testing.T, dir, name string, size int) {
	t.Helper()

	path := filepath.Join(dir, name)

	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	_, err = io.CopyN(f, rand.Reader, int64(size))
	require.NoError(t, err)
}

func TestGetModelSize(t *testing.T) {
	tests := []struct {
		name         string
		files        map[string]int
		expectedSize uint64
		expectError  bool
		setupExtra   func(t *testing.T, baseDir string)
	}{
		{
			name:         "empty directory",
			files:        map[string]int{},
			expectedSize: 0,
			expectError:  false,
		},
		{
			name: "single small file",
			files: map[string]int{
				"file1.bin": 1024 * 1024, // 1 MB
			},
			expectedSize: 1024 * 1024,
			expectError:  false,
		},
		{
			name: "multiple files",
			files: map[string]int{
				"file1.bin": 1024 * 1024, // 1 MB
				"file2.bin": 512 * 1024,  // 0.5 MB
			},
			expectedSize: 1024*1024 + 512*1024,
			expectError:  false,
		},
		{
			name:         "non existing directory",
			files:        nil,
			expectedSize: 0,
			expectError:  true,
		},
		{
			name: "permission denied on subdirectory",
			files: map[string]int{
				"allowed.bin": 1024,
			},
			expectError: true,
			setupExtra: func(t *testing.T, baseDir string) {
				restrictedDir := filepath.Join(baseDir, "private")
				err := os.Mkdir(restrictedDir, 0000)
				require.NoError(t, err)
				t.Cleanup(func() {
					_ = os.Chmod(restrictedDir, 0600)
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dir string

			if !tt.expectError {
				tmpDir := t.TempDir()
				dir = tmpDir

				for name, size := range tt.files {
					createFileWithSize(t, tmpDir, name, size)
				}

				if tt.setupExtra != nil {
					tt.setupExtra(t, tmpDir)
				}
			} else {
				dir = "/path/that/does/not/exist"
			}

			dirPath := paths.New(dir)

			sizeMB, err := getModelSize(dirPath)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expectedSize, sizeMB)
		})
	}
}
