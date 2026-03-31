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

package bricksindex

import (
	"os"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/peripherals"
)

func TestGenerateBricksIndexFromFile(t *testing.T) {
	index, err := Load(paths.New("testdata"))
	require.NoError(t, err)

	// Check if ports are correctly set
	bWebUI, found := index.FindBrickByID("arduino:web_ui")
	require.True(t, found)
	require.Equal(t, []string{"7000"}, bWebUI.Ports)

	// Check if variables are correctly set
	bIC, found := index.FindBrickByID("arduino:image_classification")
	require.True(t, found)
	require.Equal(t, "Image Classification", bIC.Name)
	require.Equal(t, "mobilenet-image-classification", bIC.ModelName)
	require.Len(t, bIC.Variables, 2)
	require.Equal(t, "CUSTOM_MODEL_PATH", bIC.Variables[0].Name)
	require.Equal(t, "/opt/models/ei/", bIC.Variables[0].DefaultValue)
	require.Equal(t, "path to the custom model directory", bIC.Variables[0].Description)
	require.Equal(t, "EI_CLASSIFICATION_MODEL", bIC.Variables[1].Name)
	require.Equal(t, "/models/ootb/ei/mobilenet-v2-224px.eim", bIC.Variables[1].DefaultValue)
	require.Equal(t, "path to the model file", bIC.Variables[1].Description)
	require.False(t, bIC.Variables[0].IsRequired())
	require.False(t, bIC.Variables[1].IsRequired())

	bRequireModel, found := index.FindBrickByID("arduino:model_required")
	require.True(t, found)
	require.True(t, bRequireModel.RequireModel)

	bDb, found := index.FindBrickByID("arduino:dbstorage_tsstore")
	require.True(t, found)
	require.False(t, bDb.RequireModel)

	bNoRequireModel, found := index.FindBrickByID("arduino:missing-model-require")
	require.True(t, found)
	require.False(t, bNoRequireModel.RequireModel)

	withHidden, found := index.FindBrickByID("arduino:with-hidden-variables")
	require.True(t, found)
	require.Equal(t, "HIDDEN_VARIABLE", withHidden.Variables[0].Name)
	require.True(t, withHidden.Variables[0].Hidden)
	require.Equal(t, "VISIBLE_VARIABLE", withHidden.Variables[1].Name)
	require.False(t, withHidden.Variables[1].Hidden)
	require.Equal(t, "VISIBLE_VARIABLE_IF_MISSING_HIDDEN", withHidden.Variables[2].Name)
	require.False(t, withHidden.Variables[2].Hidden)
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
					RequiredDevices:           []peripherals.DeviceClass{peripherals.CameraClass},
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
