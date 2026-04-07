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

		id, err := RemoveSketchLibrary(context.Background(), pythonApp, LibraryReleaseID{}, true)
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
