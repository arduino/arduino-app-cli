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
	"path/filepath"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"go.bug.st/f"
)

func TestLoad(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		app, err := Load("")
		assert.Error(t, err)
		assert.Empty(t, app)
	})

	t.Run("AppSimple", func(t *testing.T) {
		app, err := Load("testdata/AppSimple")
		assert.NoError(t, err)
		assert.NotEmpty(t, app)

		assert.NotNil(t, app.MainPythonFile)
		assert.Equal(t, f.Must(filepath.Abs("testdata/AppSimple/python/main.py")), app.MainPythonFile.String())

		assert.NotNil(t, app.MainSketchPath)
		assert.Equal(t, f.Must(filepath.Abs("testdata/AppSimple/sketch")), app.MainSketchPath.String())
	})
}

func TestMissingDescriptor(t *testing.T) {
	appFolderPath := paths.New("testdata", "MissingDescriptor")

	// Load app
	app, err := Load(appFolderPath.String())
	assert.Error(t, err)
	assert.ErrorContains(t, err, "descriptor app.yaml file missing from app")
	assert.Empty(t, app)
}

func TestMissingMains(t *testing.T) {
	appFolderPath := paths.New("testdata", "MissingMains")

	// Load app
	app, err := Load(appFolderPath.String())
	assert.Error(t, err)
	assert.ErrorContains(t, err, "main python file and sketch file missing from app")
	assert.Empty(t, app)
}
