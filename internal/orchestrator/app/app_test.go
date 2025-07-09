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
