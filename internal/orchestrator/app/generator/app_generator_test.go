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
		options    Opts
		goldenPath string
	}{
		{
			name:       "generate complete app",
			options:    None,
			goldenPath: "testdata/app-all.golden",
		},
		{
			name:       "skip sketch",
			options:    SkipSketch,
			goldenPath: "testdata/app-no-sketch.golden",
		},
		{
			name:       "skip python",
			options:    SkipPython,
			goldenPath: "testdata/app-no-python.golden",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()

			err := GenerateApp(paths.New(tempDir), baseApp, tc.options)
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
