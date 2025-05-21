package parser

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
)

func TestAppParser(t *testing.T) {
	// Test a simple app descriptor file
	appPath := paths.New("testdata", "app.yaml")
	app, err := ParseDescriptorFile(appPath)
	require.NoError(t, err)

	require.Equal(t, app.Name, "Image detection with UI")
	require.Equal(t, app.Ports[0], 7860)

	brick1 := Brick{
		Name:  "arduino/object_detection",
		Model: "vision/yolo11",
	}
	require.Contains(t, app.Bricks, brick1)

	// Test a more complex app descriptor file, with additional bricks
	appPath = paths.New("testdata", "complex-app.yaml")
	app, err = ParseDescriptorFile(appPath)
	require.NoError(t, err)

	require.Equal(t, app.Name, "Complex app")
	require.Contains(t, app.Ports, 7860, 8080)

	brick2 := Brick{
		Name: "arduino/not_found",
	}
	brick3 := Brick{
		Name: "arduino/simple_string",
	}
	require.Contains(t, app.Bricks, brick1, brick2, brick3)

	// Test a case that should fail.
	appPath = paths.New("testdata", "wrong-app.yaml")
	app, err = ParseDescriptorFile(appPath)
	require.Error(t, err)
}
