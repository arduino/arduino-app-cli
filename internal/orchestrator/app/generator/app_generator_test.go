// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package generator

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

func TestGenerateApp(t *testing.T) {
	baseApp := app.AppDescriptor{
		Name:        "test app all",
		Description: "test description.",
		Icon:        "🚀",
		Ports:       []int{8080, 9000, 90},
	}

	testCases := []struct {
		name       string
		skipSketch bool
		goldenPath string
	}{
		{
			name:       "generate complete app",
			goldenPath: "testdata/app-all.golden",
		},
		{
			name:       "skip sketch",
			skipSketch: true,
			goldenPath: "testdata/app-no-sketch.golden",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()

			err := GenerateApp(paths.New(tempDir), baseApp, tc.skipSketch)
			require.NoError(t, err)

			if os.Getenv("UPDATE_GOLDEN") == "true" {
				t.Logf("UPDATE_GOLDEN=true: updating  golden files in %s", tc.goldenPath)
				require.NoError(t, os.RemoveAll(tc.goldenPath))
				require.NoError(t, os.CopyFS(tc.goldenPath, os.DirFS(tempDir)))
			} else {
				compareFolders(t, paths.New(tempDir), paths.New(tc.goldenPath))
			}
		})
	}
}

func TestGenerateAppWithBricks(t *testing.T) {
	tempDir := paths.New(t.TempDir())
	descriptor := app.AppDescriptor{
		Name: "test app with bricks",
		Bricks: []app.Brick{
			{ID: "arduino:air_quality_monitoring"},
			{ID: "arduino:object_detection"},
		},
	}

	require.NoError(t, GenerateApp(tempDir, descriptor, true))
	generated := f.Must(app.Load(tempDir))
	require.Equal(t, descriptor.Bricks, generated.Descriptor.Bricks)
}

func compareFolders(t *testing.T, actualPath, goldenPath *paths.Path) {
	t.Helper()

	getFiles := func(root string) ([]string, error) {
		var files []string
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return fmt.Errorf("failed to retrieve relative path for %q: %w", path, err)
			}
			files = append(files, rel)
			return nil
		})
		return files, err
	}

	goldenDir := goldenPath.String()
	actualDir := actualPath.String()

	goldenFiles, err := getFiles(goldenDir)
	require.NoError(t, err, "failed reading golden dir")

	actualFiles, err := getFiles(actualDir)
	require.NoError(t, err, "failed reading actual dir")

	sort.Strings(goldenFiles)
	sort.Strings(actualFiles)

	require.Equal(t, goldenFiles, actualFiles, "golden dir and actual dir should have the same structure")

	for _, relPath := range goldenFiles {
		goldenContent, err := os.ReadFile(filepath.Join(goldenDir, relPath))
		require.NoError(t, err, "failed reading golden file: %q", relPath)
		actualContent, err := os.ReadFile(filepath.Join(actualDir, relPath))
		require.NoError(t, err, "failed reading actual file: %s", relPath)
		require.True(t, bytes.Equal(goldenContent, actualContent), "content should be the same: %q", relPath)

	}
}

func TestGenerateAppBrick(t *testing.T) {
	t.Run("generate local brick for an app", func(t *testing.T) {
		appDir := paths.New(t.TempDir())

		err := GenerateApp(appDir, app.AppDescriptor{Name: "an-app-with-brick"}, true)
		require.NoError(t, err)

		a := f.Must(app.Load(appDir))

		err = GenerateLocalBrick(a.GetBricksPath(), "my-brick", "a-brick-name")
		require.NoError(t, err)
		if os.Getenv("UPDATE_GOLDEN") == "true" {
			t.Logf("UPDATE_GOLDEN=true: updating  golden files in %s", "testdata/app-with-brick.golden")
			require.NoError(t, os.RemoveAll("testdata/app-with-brick.golden"))
			require.NoError(t, os.CopyFS("testdata/app-with-brick.golden", os.DirFS(appDir.String())))
		} else {
			compareFolders(t, appDir, paths.New("testdata/app-with-brick.golden"))
		}
	})

	t.Run("conflict if the folder already exist", func(t *testing.T) {
		tmpDir := paths.New(t.TempDir())
		err := tmpDir.Join("my-brick").MkdirAll()
		require.NoError(t, err)

		err = GenerateLocalBrick(tmpDir, "my-brick", "a-brick-name")
		require.ErrorIs(t, err, ErrBrickAlreadyExists)
	})
}

func TestBrickIDFromName(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedID   string
		errorMessage string
	}{
		{
			name:       "simple name",
			input:      "MyBrick",
			expectedID: "mybrick",
		},
		{
			name:       "name with spaces",
			input:      "My Awesome Brick",
			expectedID: "my_awesome_brick",
		},
		{
			name:       "name with hyphens",
			input:      "my-brick",
			expectedID: "my_brick",
		},
		{
			name:       "name with numbers",
			input:      "Brick123",
			expectedID: "brick123",
		},
		{
			name:       "name with mixed case and special chars",
			input:      "My-Awesome_Brick",
			expectedID: "my_awesome_brick",
		},
		{
			name:       "name with leading/trailing spaces",
			input:      "  MyBrick  ",
			expectedID: "mybrick",
		},
		{
			name:       "single character name",
			input:      "A",
			expectedID: "a",
		},
		{
			name:         "only special characters",
			input:        "---___",
			expectedID:   "",
			errorMessage: "brick name must contain at least one alphanumeric character",
		},
		{
			name:       "mixed valid and underscore",
			input:      "my_brick",
			expectedID: "my_brick",
		},
		{
			name:       "replaces multiple spaces with single underscore",
			input:      "my   brick",
			expectedID: "my_brick",
		},
		{
			name:       "trims leading underscores",
			input:      "___mybrick",
			expectedID: "mybrick",
		},
		{
			name:       "trims trailing underscores",
			input:      "mybrick___",
			expectedID: "mybrick",
		},
		{
			name:       "keeps internal alphanumerics and underscores",
			input:      "my_brick_123",
			expectedID: "my_brick_123",
		},
		{
			name:       "numbers only",
			input:      "123456",
			expectedID: "123456",
		},
		{
			name:       "with special chars that get converted",
			input:      "my@brick#test",
			expectedID: "my_brick_test",
		},
		{
			name:         "name with dot character",
			input:        "my.brick",
			expectedID:   "",
			errorMessage: "brick name cannot contain '.' character",
		},
		{
			name:         "name with dot at the beginning",
			input:        ".mybrick",
			expectedID:   "",
			errorMessage: "brick name cannot contain '.' character",
		},
		{
			name:         "name with a version",
			input:        "My Awesome Brick v2.0",
			expectedID:   "",
			errorMessage: "brick name cannot contain '.' character",
		},
		{
			name:         "name with colon character",
			input:        "my:brick",
			expectedID:   "",
			errorMessage: "brick name cannot contain ':' character",
		},
		{
			name:         "name with colon at the end",
			input:        "mybrick:",
			expectedID:   "",
			errorMessage: "brick name cannot contain ':' character",
		},
		{
			name:         "dot before colon",
			input:        "test.name:brick",
			expectedID:   "",
			errorMessage: "brick name cannot contain '.' character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := BrickIDFromName(tt.input)

			if tt.errorMessage != "" {
				require.Error(t, err, "expected error but got none")
				require.Equal(t, tt.errorMessage, err.Error(), "error message mismatch")
			} else {
				require.NoError(t, err, "expected no error but got: %v", err)
				require.Equal(t, tt.expectedID, id, "generated ID mismatch")
			}
		})
	}
}
