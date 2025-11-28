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
	"context"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

func TestListSketchLibraries(t *testing.T) {
	t.Run("fail to list libraries if the sketch folder is missing", func(t *testing.T) {
		pythonApp, err := app.Load(createTestAppPythonOnly(t))
		require.NoError(t, err)

		libs, err := ListSketchLibraries(context.Background(), pythonApp)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot list libraries. Missing sketch folder")
		assert.Empty(t, libs)
	})

	t.Run("fail to add library if the sketch folder is missing", func(t *testing.T) {
		pythonApp, err := app.Load(createTestAppPythonOnly(t))
		require.NoError(t, err)

		libs, err := AddSketchLibrary(context.Background(), pythonApp, LibraryReleaseID{}, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot add a library. Missing sketch folder")
		assert.Empty(t, libs)
	})

	t.Run("fail to remove library if the sketch folder is missing", func(t *testing.T) {
		pythonApp, err := app.Load(createTestAppPythonOnly(t))
		require.NoError(t, err)

		id, err := RemoveSketchLibrary(context.Background(), pythonApp, LibraryReleaseID{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot remove a library. Missing sketch folder")
		assert.Empty(t, id)
	})
}

// Helper function to create a test app without sketch path (Python-only)
func createTestAppPythonOnly(t *testing.T) *paths.Path {
	tempDir := t.TempDir()

	appYaml := paths.New(tempDir, "app.yaml")
	require.NoError(t, appYaml.WriteFile([]byte(`
name: test-python-app
version: 1.0.0
description: Test Python-only app
`)))

	// Create python directory and file
	pythonDir := paths.New(tempDir, "python")
	require.NoError(t, pythonDir.MkdirAll())

	pythonFile := pythonDir.Join("main.py")
	require.NoError(t, pythonFile.WriteFile([]byte(`
import time

def main():
    print("Hello from Python!")
    time.sleep(1)

if __name__ == "__main__":
    main()
`)))
	return paths.New(tempDir)
}
