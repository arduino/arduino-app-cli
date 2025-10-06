package app

import (
	"os"
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
		ID:    "arduino:object_detection",
		Model: "vision/yolo11",
		Variables: map[string]string{
			"PORT":          "8080",
			"ROOT_PASSWORD": "secret",
		},
	}
	require.Contains(t, app.Bricks, brick1)

	// Test a more complex app descriptor file, with additional bricks
	appPath = paths.New("testdata", "complex-app.yaml")
	app, err = ParseDescriptorFile(appPath)
	require.NoError(t, err)

	require.Equal(t, app.Name, "Complex app")
	require.Contains(t, app.Ports, 7860, 8080)

	brick2 := Brick{
		ID: "arduino:not_found",
	}
	brick3 := Brick{
		ID: "arduino:simple_string",
	}
	require.Contains(t, app.Bricks, brick1, brick2, brick3)

	// Test a case that should fail.
	appPath = paths.New("testdata", "wrong-app.yaml")
	app, err = ParseDescriptorFile(appPath)
	require.Error(t, err)
}

func TestIsSingleEmoji(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"😃", true},
		{"👩🏼‍🚀", true},
		{"😃😃", false},
		{"not", false},
		{"", false},
		{"👩🏼‍🚀👩🏼‍🚀", false},
		{"👩🏼‍🚀n", false},
		{"n👩🏼‍🚀", false},
		{"👩🏼‍🚀😃", false},
		{"⚡", true},
		{"⚡️", true}, // High Voltage + Varinat Selector 16 (ref: https://en.wikipedia.org/wiki/Variation_Selectors_(Unicode_block))
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result := isSingleEmoji(test.input)
			require.Equal(t, test.expected, result, "Input: %s", test.input)
		})
	}
}

func TestArduinoApp_Load(t *testing.T) {
	tempDir := t.TempDir()
	err := paths.New(tempDir).MkdirAll()
	require.NoError(t, err)

	// Create minimal setup
	err = paths.New(tempDir, "python").MkdirAll()
	require.NoError(t, err)
	err = os.WriteFile(paths.New(tempDir, "python", "main.py").String(), []byte("print('Hello World')"), 0600)
	require.NoError(t, err)
	// Create a valid app.yaml file
	appYaml := paths.New(tempDir, "app.yaml")

	appDescriptor :=
		`name: Test App
bricks:
  - arduino:object_detection:
      model: yolox-object-detection
      variables:
        "EI_OBJ_DETECTION_MODEL": "/home/arduino/.arduino-bricks/ei-models/face-det.eim"
`

	err = os.WriteFile(appYaml.String(), []byte(appDescriptor), 0600)
	require.NoError(t, err)

	app, err := Load(tempDir)
	require.NoError(t, err)
	require.Equal(t, "Test App", app.Name)
	require.Equal(t, 1, len(app.Descriptor.Bricks))
	require.Equal(t, "arduino:object_detection", app.Descriptor.Bricks[0].ID)
	require.Equal(t, "yolox-object-detection", app.Descriptor.Bricks[0].Model)
	require.Equal(t, "/home/arduino/.arduino-bricks/ei-models/face-det.eim", app.Descriptor.Bricks[0].Variables["EI_OBJ_DETECTION_MODEL"])
}
