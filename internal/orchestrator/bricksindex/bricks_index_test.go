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

package bricksindex

import (
	"os"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
)

func TestGenerateBricksIndexFromFile(t *testing.T) {
	index, err := Load(paths.New("testdata"))
	require.NoError(t, err)

	// Check if ports are correctly set
	b, found := index.FindBrickByID("arduino:web_ui")
	require.True(t, found)
	require.Equal(t, []string{"7000"}, b.Ports)

	// Check if variables are correctly set
	b, found = index.FindBrickByID("arduino:image_classification")
	require.True(t, found)
	require.Equal(t, "Image Classification", b.Name)
	require.Equal(t, "mobilenet-image-classification", b.ModelName)
	require.True(t, b.RequireModel)
	require.Len(t, b.Variables, 2)
	require.Equal(t, "CUSTOM_MODEL_PATH", b.Variables[0].Name)
	require.Equal(t, "/opt/models/ei/", b.Variables[0].DefaultValue)
	require.Equal(t, "path to the custom model directory", b.Variables[0].Description)
	require.Equal(t, "EI_CLASSIFICATION_MODEL", b.Variables[1].Name)
	require.Equal(t, "/models/ootb/ei/mobilenet-v2-224px.eim", b.Variables[1].DefaultValue)
	require.Equal(t, "path to the model file", b.Variables[1].Description)
	require.False(t, b.Variables[0].IsRequired())
	require.False(t, b.Variables[1].IsRequired())
}

func TestBricksIndexYAMLFormats(t *testing.T) {
	testCases := []struct {
		name           string
		yamlContent    string
		expectedError  string
		expectedBricks []Brick
	}{
		{
			// TODO: add a validator fo the bricks-list to validate the field
			name:           "missing bricks field does not cuase error",
			yamlContent:    `other_field: value`,
			expectedBricks: nil,
		},
		{
			name: "bad YAML format invalid indentation",
			yamlContent: `bricks:
		- id: arduino:test_brick
		name: Test Brick
		  description: A test brick`,
			expectedError: "found character '\t' that cannot start any token",
		},
		{
			name:           "empty bricks",
			yamlContent:    `bricks: []`,
			expectedBricks: []Brick{},
		},
		{
			name: "bad YAML format unclosed quotes",
			yamlContent: `bricks:
- id: "arduino:test_brick
  name: Test Brick
  description: A test brick`,
			expectedError: "could not find end character of double-quoted text",
		},
		{
			name: "bad YAML format missing colon",
			yamlContent: `bricks:
- id arduino:test_brick
  name: Test Brick`,
			expectedError: "unexpected key name",
		},
		{
			name: "bad YAML format invalid syntax",
			yamlContent: `bricks:
- id: arduino:test_brick
  name: Test Brick
  description: A test brick
  ports: [7000,`,
			expectedError: "sequence end token ']' not found",
		},
		{
			name:          "bad YAML format tab characters",
			yamlContent:   "bricks:\n\t- id: arduino:test_brick\n\t  name: Test Brick",
			expectedError: "found character '\t' that cannot start any token",
		},
		{
			name: "simple brick",
			yamlContent: `bricks:
- id: arduino:simple_brick
  name: Test Brick
  description: A test brick
`,
			expectedBricks: []Brick{
				{
					ID:                        "arduino:simple_brick",
					Name:                      "Test Brick",
					Description:               "A test brick",
					Category:                  "",
					RequiresDisplay:           "",
					RequireContainer:          false,
					RequireModel:              false,
					RequiredDevices:           nil,
					Variables:                 nil,
					Ports:                     nil,
					ModelName:                 "",
					MountDevicesIntoContainer: false,
				},
			},
		},
		{
			name: "valid YAML with complex variables",
			yamlContent: `bricks:
- id: arduino:complex_brick
  name: Complex Brick
  description: A complex test brick
  category: storage
  require_container: true
  require_model: true
  require_devices: false
  mount_devices_into_container: true
  model_name: a-complex-model
  required_devices:
  - camera
  ports:
  - 7000
  - 8080
  variables:
  - name: REQUIRED_VAR
    default_value: ""
    description: A required variable
  - name: OPTIONAL_VAR
    default_value: "default_value"
    description: An optional variable`,
			expectedBricks: []Brick{
				{
					ID:                        "arduino:complex_brick",
					Name:                      "Complex Brick",
					Description:               "A complex test brick",
					Category:                  "storage",
					RequiresDisplay:           "",
					RequireContainer:          true,
					RequireModel:              true,
					RequiredDevices:           []string{"camera"},
					MountDevicesIntoContainer: true,
					Variables: []BrickVariable{
						{
							Name:         "REQUIRED_VAR",
							DefaultValue: "",
							Description:  "A required variable",
						},
						{
							Name:         "OPTIONAL_VAR",
							DefaultValue: "default_value",
							Description:  "An optional variable",
						},
					},
					Ports:     []string{"7000", "8080"},
					ModelName: "a-complex-model",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			brickIndex := paths.New(tempDir, "bricks-list.yaml")
			err := os.WriteFile(brickIndex.String(), []byte(tc.yamlContent), 0600)
			require.NoError(t, err)

			index, err := Load(paths.New(tempDir))
			if tc.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedError)
			} else {
				require.NoError(t, err)
				require.Equal(t, index.Bricks, tc.expectedBricks, "bricsk mistmatch")
			}
		})
	}
}
