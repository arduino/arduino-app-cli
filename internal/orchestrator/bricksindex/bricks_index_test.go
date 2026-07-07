// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"os"
	"slices"
	"testing"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/peripherals"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

func TestGenerateBricksIndexFromFile(t *testing.T) {
	yamlContent := `
bricks:
- id: arduino:basic
  name: i-am-a-basic-brick
  description: i am a basic brick with only id, name and description
  category: "Image"
- id: arduino:with-ports
  ports:
  - 7000
  - 8000
- id: arduino:with-model-variables
  model_name: mobilenet-image-classification
  variables:
  - name: FIRST_VARIABLE
    default_value: default-value-for-first-variable
    description: description for the first variable
  - name: SECOND_VARIABLE
    description: description for the second variable
- id: arduino:with-model-name
  model_name: mobilenet-image-classification
- id: arduino:with-model-by-boards
  model_by_boards:
  - platform: ventunoq
    model: mobilenet-image-classification
- id: arduino:missing-model
- id: arduino:with-hidden-variables
  variables:
    - name: HIDDEN_VARIABLE
      default_value: a_hidden_value
      description: this variable is hidden
      hidden: true
    - name: VISIBLE_VARIABLE
      default_value: a_visible_value
      description: this variable is visible because 'hidden' is set to false
      hidden: false
    - name: VISIBLE_VARIABLE_IF_MISSING_HIDDEN
      default_value: another_visible_value
      description: this variable is visiable because 'hidden' field is missing
      hidden: false

`
	assetDir := paths.TempDir()
	err := assetDir.Join("bricks-list.yaml").WriteFile([]byte(yamlContent))
	require.NoError(t, err)

	index, err := Load(platform.Platform{BoardName: "ventunoq"}, assetDir)
	require.NoError(t, err)

	brickBasi, found := index.FindBrickByID("arduino:basic")
	require.True(t, found)
	require.Equal(t, "arduino:basic", brickBasi.ID)
	require.Equal(t, "i-am-a-basic-brick", brickBasi.Name)
	require.Equal(t, "i am a basic brick with only id, name and description", brickBasi.Description)
	require.Equal(t, "Image", brickBasi.Category)

	// Check if ports are correctly set
	bWithPorts, found := index.FindBrickByID("arduino:with-ports")
	require.True(t, found)
	require.Equal(t, []string{"7000", "8000"}, bWithPorts.Ports)

	// Check if variables are correctly set
	bWithVariables, found := index.FindBrickByID("arduino:with-model-variables")
	require.True(t, found)
	require.Equal(t, "mobilenet-image-classification", bWithVariables.ModelName)
	require.Len(t, bWithVariables.Variables, 2)
	require.Equal(t, "FIRST_VARIABLE", bWithVariables.Variables[0].Name)
	require.Equal(t, "default-value-for-first-variable", bWithVariables.Variables[0].DefaultValue)
	require.Equal(t, "description for the first variable", bWithVariables.Variables[0].Description)
	require.False(t, bWithVariables.Variables[0].IsRequired())
	require.Equal(t, "SECOND_VARIABLE", bWithVariables.Variables[1].Name)
	require.Equal(t, "", bWithVariables.Variables[1].DefaultValue)
	require.Equal(t, "description for the second variable", bWithVariables.Variables[1].Description)
	require.True(t, bWithVariables.Variables[1].IsRequired())

	bRequireModel, found := index.FindBrickByID("arduino:with-model-name")
	require.True(t, found)
	require.True(t, bRequireModel.RequireModel)

	bNoRequireModel, found := index.FindBrickByID("arduino:missing-model")
	require.True(t, found)
	require.False(t, bNoRequireModel.RequireModel)

	indexVentuno, err := Load(platform.Platform{BoardName: "ventunoq"}, assetDir)
	require.NoError(t, err)
	brickVentuno, found := indexVentuno.FindBrickByID("arduino:with-model-by-boards")
	require.True(t, found)
	require.True(t, brickVentuno.RequireModel)

	indexUnoQ, err := Load(platform.Platform{BoardName: "unoq"}, assetDir)
	require.NoError(t, err)
	brickUno, found := indexUnoQ.FindBrickByID("arduino:with-model-by-boards")
	require.True(t, found)
	require.False(t, brickUno.RequireModel)

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
		{
			name: "requires_services as plain string list",
			yamlContent: `bricks:
- id: arduino:simple_brick
  name: Test Brick
  description: A test brick
  requires_services:
  - arduino:genie_audio
  - arduino:other_service
`,
			expectedBricks: []Brick{
				{
					ID:          "arduino:simple_brick",
					Name:        "Test Brick",
					Description: "A test brick",
					RequiresServices: []RequiresService{
						{ID: "arduino:genie_audio"},
						{ID: "arduino:other_service"},
					},
				},
			},
		},
		{
			name: "requires_services as struct list with when condition",
			yamlContent: `bricks:
- id: arduino:simple_brick
  name: Test Brick
  description: A test brick
  requires_services:
  - id: arduino:genie
    when:
      model: genie:*
  - id: arduino:llamacpp_npu
    when:
      model: llamacpp:*
`,
			expectedBricks: []Brick{
				{
					ID:          "arduino:simple_brick",
					Name:        "Test Brick",
					Description: "A test brick",
					RequiresServices: []RequiresService{
						{ID: "arduino:genie", When: &RequiresServiceMatch{Model: new("genie:*")}},
						{ID: "arduino:llamacpp_npu", When: &RequiresServiceMatch{Model: new("llamacpp:*")}},
					},
				},
			},
		},
		{
			name: "model_by_boards with platform and model fields",
			yamlContent: `bricks:
- id: arduino:brick_with_model_by_boards
  name: Brick With Model By Boards
  description: A brick with model_by_boards
  model_name: default-model
  model_by_boards:
  - platform: ventunoq
    model: ventunoq-model
  - platform: portenta
    model: portenta-model
`,
			expectedBricks: []Brick{
				{
					ID:           "arduino:brick_with_model_by_boards",
					Name:         "Brick With Model By Boards",
					Description:  "A brick with model_by_boards",
					ModelName:    "default-model",
					RequireModel: true,
					ModelByBoard: []ModelsBoard{
						{Platform: "ventunoq", Model: "ventunoq-model"},
						{Platform: "portenta", Model: "portenta-model"},
					},
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

			index, err := Load(platform.GetPlatform(nil), paths.New(tempDir))
			if tc.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedError)
			} else {
				require.NoError(t, err)
				assert.Empty(t, cmp.Diff(index.BuiltInBricks, tc.expectedBricks, cmpopts.IgnoreFields(Brick{}, "FullPath", "Source", "ComposeFile", "ReadmeFile", "ExamplesPath", "DocsAPIPath", "containerPorts")))
			}
		})
	}
}

func TestLoadBrickYamlBrickIndex(t *testing.T) {

	t.Run("get files of a brick in the yaml index", func(t *testing.T) {
		bricksIndex, err := Load(platform.GetPlatform(nil), paths.New("testdata/0.4.8"))
		require.NoError(t, err)

		brick, found := bricksIndex.FindBrickByID("arduino:a-good-brick")
		require.True(t, found)
		assert.Equal(t, paths.New("testdata/0.4.8"), brick.FullPath)

		content, err := brick.GetReadmeFile()
		require.NoError(t, err)
		require.Equal(t, "# i-am-a-readme-file", content)

		compose, found := brick.GetComposeFile()
		require.True(t, found)
		require.Equal(t, paths.New("testdata/0.4.8/compose/arduino/a-good-brick/brick_compose.yaml"), compose)

		examples, err := brick.GetExamplesPath()
		require.NoError(t, err)
		require.Equal(t, paths.NewPathList("testdata/0.4.8/examples/arduino/a-good-brick/example_1.py", "testdata/0.4.8/examples/arduino/a-good-brick/example_2.py"), examples)

		api, found := brick.GetApiDocPath()
		require.True(t, found)
		require.Equal(t, paths.New("testdata/0.4.8/api-docs/arduino/app_bricks/a-good-brick/API.md"), api)

		ports := brick.GetPorts()
		require.Equal(t, []string{"6000", "8080"}, ports)
	})

	t.Run("find a brick in local bricks", func(t *testing.T) {
		bricksIndex, err := Load(platform.GetPlatform(nil), paths.New("testdata/0.4.8"))
		require.NoError(t, err)
		brickWithLoca := bricksIndex.WithAppBricks([]Brick{{ID: "my-first-brick", Source: "another-source"}})
		brick, found := brickWithLoca.FindBrickByID("my-first-brick")
		require.True(t, found)
		require.Equal(t, "my-first-brick", brick.ID)
		require.Equal(t, "another-source", brick.Source)
	})

	t.Run("local brick has priority to yaml index", func(t *testing.T) {
		bricksIndex, err := Load(platform.GetPlatform(nil), paths.New("testdata/0.4.8"))
		require.NoError(t, err)
		brickWithLoca := bricksIndex.WithAppBricks([]Brick{{ID: "arduino:a-good-brick", Source: "another-source"}})
		brick, found := brickWithLoca.FindBrickByID("arduino:a-good-brick")
		require.True(t, found)
		require.Equal(t, "arduino:a-good-brick", brick.ID)
		require.Equal(t, "another-source", brick.Source)
	})

	t.Run("get a brick for supported board", func(t *testing.T) {
		t.Run("no-board", func(t *testing.T) {
			bricksIndex, err := Load(platform.Platform{BoardName: ""}, paths.New("testdata/0.4.8"))
			require.NoError(t, err)
			bricks := bricksIndex.ListBricks()
			require.Len(t, bricks, 3)
		})

		t.Run("foo-board", func(t *testing.T) {
			platform := platform.Platform{BoardName: "foo-board"}
			bricksIndex, err := Load(platform, paths.New("testdata/0.4.8"))
			require.NoError(t, err)
			bricks := bricksIndex.ListBricks()
			require.Len(t, bricks, 3)
		})
		t.Run("another-board", func(t *testing.T) {
			platform := platform.Platform{BoardName: "another-board"}
			bricksIndex, err := Load(platform, paths.New("testdata/0.4.8"))
			require.NoError(t, err)
			bricks := bricksIndex.ListBricks()
			require.Len(t, bricks, 2)
		})
	})

	t.Run("get by platform compose files", func(t *testing.T) {
		// Force a Ventunoq platform to test the retrieval of platform-specific compose file
		ventunoq := platform.Platform{
			FQBN:        "arduino:zephyr:ventunoq",
			PlatformID:  "arduino:zephyr",
			BoardName:   "ventunoq",
			CompileJobs: 0, // unlimited
		}
		bricksIndex, err := Load(ventunoq, paths.New("testdata/0.4.8"))
		require.NoError(t, err)

		brick, found := bricksIndex.FindBrickByID("arduino:a-good-brick-by-platform")
		require.True(t, found)
		assert.Equal(t, paths.New("testdata/0.4.8"), brick.FullPath)

		compose, found := brick.GetComposeFile()
		require.True(t, found)
		require.Equal(t, paths.New("testdata/0.4.8/compose/arduino/a-good-brick-by-platform/brick_compose.ventunoq.yaml"), compose)

	})
}

func TestListBricksSupportedBoard(t *testing.T) {
	brick1 := Brick{ID: "foo:1", Name: "brick1", SupportedBoards: nil}
	brick2 := Brick{ID: "foo:2", Name: "brick2", SupportedBoards: []string{"foo"}}
	brick3 := Brick{ID: "foo:3", Name: "brick3", SupportedBoards: []string{"foo", "bar"}}

	tests := []struct {
		name       string
		platform   platform.Platform
		wantBricks []Brick
	}{
		{
			name:       "all bricks supported when no board specified",
			platform:   platform.Platform{BoardName: ""},
			wantBricks: []Brick{brick1, brick2, brick3},
		},
		{
			name:       "all foo bricks and bricks without supported board specified",
			platform:   platform.Platform{BoardName: "foo"},
			wantBricks: []Brick{brick1, brick2, brick3},
		},
		{
			name:       "only bar bricks and bricks without supported board specified",
			platform:   platform.Platform{BoardName: "bar"},
			wantBricks: []Brick{brick1, brick3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brickYaml := YamlBricksIndex{
				Bricks: []Brick{brick1, brick2, brick3},
			}
			tmpDir := paths.New(t.TempDir())
			f, err := tmpDir.Join("bricks-list.yaml").Create()
			require.NoError(t, err)
			err = yaml.NewEncoder(f).Encode(brickYaml)
			require.NoError(t, err)
			err = f.Close()
			require.NoError(t, err)

			b, err := Load(tt.platform, tmpDir)
			require.NoError(t, err)

			got := b.ListBricks()
			for i := range got {
				require.Equal(t, tt.wantBricks[i].ID, got[i].ID)
			}
		})
	}
}

func TestExtractPortsFromComposeFile(t *testing.T) {
	testCases := []struct {
		name      string
		content   string
		want      []string
		expectErr bool
	}{
		{
			name: "basic",
			content: `
version: "3"
services:
  web:
    ports:
      - "8080:80"
      - "443:443"
  db:
    ports:
      - "5432"
      - "127.0.0.1:15432:5432"
  cache:
    ports:
      - "6379"
      - "6380:6380"
  multi:
    ports:
      - "0.0.0.0:9000:9000/tcp"
      - "10000:10000"
`,
			want:      []string{"8080", "443", "5432", "15432", "6379", "6380", "9000", "10000"},
			expectErr: false,
		},
		{
			name: "no_ports",
			content: `
version: "3"
services:
  web:
    image: nginx
  db:
    image: postgres
`,
			want:      nil,
			expectErr: false,
		},
		{
			name: "invalid_yaml",
			content: `
version: "3"
services
  web:
    ports: [8080:80]
`,
			want:      nil,
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpFile := paths.New(t.TempDir()).Join("compose.yaml")
			err := tmpFile.WriteFile([]byte(tc.content))
			require.NoError(t, err)

			got, err := extractPortsFromComposeFile(tmpFile)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				slices.Sort(tc.want)
				slices.Sort(got)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestGetMatchingService(t *testing.T) {
	tests := []struct {
		name             string
		requiresServices []RequiresService
		brickInstance    BrickInstance
		wantServices     []string
		wantErr          bool
	}{
		{
			name:             "no requires_services returns empty slice",
			requiresServices: nil,
			brickInstance:    BrickInstance{Model: "some-model"},
			wantServices:     []string{},
		},
		{
			name: "service with no when condition matches unconditionally",
			requiresServices: []RequiresService{
				{ID: "service-a", When: nil},
			},
			brickInstance: BrickInstance{Model: "any-model"},
			wantServices:  []string{"service-a"},
		},
		{
			name: "service with when but no model field matches unconditionally",
			requiresServices: []RequiresService{
				{ID: "service-b", When: &RequiresServiceMatch{Model: nil}},
			},
			brickInstance: BrickInstance{Model: "any-model"},
			wantServices:  []string{"service-b"},
		},
		{
			name: "service with matching model pattern returns service ID",
			requiresServices: []RequiresService{
				{ID: "service-c", When: &RequiresServiceMatch{Model: new("mobilenet-*")}},
			},
			brickInstance: BrickInstance{Model: "mobilenet-image-classification"},
			wantServices:  []string{"service-c"},
		},
		{
			name: "service with non-matching model pattern returns empty slice",
			requiresServices: []RequiresService{
				{ID: "service-d", When: &RequiresServiceMatch{Model: new("mobilenet-*")}},
			},
			brickInstance: BrickInstance{Model: "yolo-object-detection"},
			wantServices:  []string{},
		},
		{
			name: "only second service matches",
			requiresServices: []RequiresService{
				{ID: "service-e", When: &RequiresServiceMatch{Model: new("mobilenet-*")}},
				{ID: "service-f", When: &RequiresServiceMatch{Model: new("yolo-*")}},
			},
			brickInstance: BrickInstance{Model: "yolo-object-detection"},
			wantServices:  []string{"service-f"},
		},
		{
			name: "multiple services match and all are returned",
			requiresServices: []RequiresService{
				{ID: "service-g", When: &RequiresServiceMatch{Model: new("mobilenet-*")}},
				{ID: "service-h", When: nil},
			},
			brickInstance: BrickInstance{Model: "mobilenet-image-classification"},
			wantServices:  []string{"service-g", "service-h"},
		},
		{
			name: "no service matches returns empty slice",
			requiresServices: []RequiresService{
				{ID: "service-i", When: &RequiresServiceMatch{Model: new("mobilenet-*")}},
				{ID: "service-j", When: &RequiresServiceMatch{Model: new("yolo-*")}},
			},
			brickInstance: BrickInstance{Model: "resnet-classification"},
			wantServices:  []string{},
		},
		{
			name: "invalid pattern returns error",
			requiresServices: []RequiresService{
				{ID: "service-k", When: &RequiresServiceMatch{Model: new("[invalid")}},
			},
			brickInstance: BrickInstance{Model: "any-model"},
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brick := Brick{RequiresServices: tt.requiresServices}
			got, err := brick.GetMatchingService(tt.brickInstance)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantServices, got)
			}
		})
	}
}

func TestRequiresServicesUnmarshalYAML(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		want        RequiresServices
		wantErr     bool
		errContains string
	}{
		{
			name: "list of plain strings",
			yaml: `
- arduino:genie_audio
- arduino:other_service
`,
			want: RequiresServices{
				{ID: "arduino:genie_audio"},
				{ID: "arduino:other_service"},
			},
		},
		{
			name: "list of structs with when condition",
			yaml: `
- id: arduino:genie
  when:
    model: genie:*
- id: arduino:llamacpp_npu
  when:
    model: llamacpp:*
`,
			want: RequiresServices{
				{ID: "arduino:genie", When: &RequiresServiceMatch{Model: new("genie:*")}},
				{ID: "arduino:llamacpp_npu", When: &RequiresServiceMatch{Model: new("llamacpp:*")}},
			},
		},
		{
			name: "mixed plain strings and structs",
			yaml: `
- arduino:genie_audio
- id: arduino:genie
  when:
    model: genie:*
`,
			want: RequiresServices{
				{ID: "arduino:genie_audio"},
				{ID: "arduino:genie", When: &RequiresServiceMatch{Model: new("genie:*")}},
			},
		},
		{
			name: "struct with no when condition",
			yaml: `
- id: arduino:genie
`,
			want: RequiresServices{
				{ID: "arduino:genie"},
			},
		},
		{
			name:        "not a sequence",
			yaml:        `arduino:genie_audio`,
			wantErr:     true,
			errContains: "requires_services: expected a sequence",
		},
		{
			name: "unexpected node type in sequence",
			yaml: `
- - nested
`,
			wantErr:     true,
			errContains: "requires_services: unexpected node type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got RequiresServices
			err := yaml.Unmarshal([]byte(tt.yaml), &got)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestLoadResolvesModelNameByBoard(t *testing.T) {
	yamlContent := `bricks:
- id: arduino:brick-a
  name: Brick A
  model_name: default-model
  model_by_boards:
  - platform: ventunoq
    model: ventunoq-model
  - platform: portenta
    model: portenta-model
- id: arduino:brick-b
  name: Brick B
  model_name: only-default
`

	tests := []struct {
		name            string
		boardName       string
		wantBrickAModel string
		wantBrickBModel string
	}{
		{
			name:            "matching platform overrides default",
			boardName:       "ventunoq",
			wantBrickAModel: "ventunoq-model",
			wantBrickBModel: "only-default",
		},
		{
			name:            "board not in list keeps default",
			boardName:       "opta",
			wantBrickAModel: "default-model",
			wantBrickBModel: "only-default",
		},
		{
			name:            "empty board keeps default",
			boardName:       "",
			wantBrickAModel: "default-model",
			wantBrickBModel: "only-default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			brickIndex := paths.New(tempDir, "bricks-list.yaml")
			require.NoError(t, os.WriteFile(brickIndex.String(), []byte(yamlContent), 0600))

			index, err := Load(platform.Platform{BoardName: tt.boardName}, paths.New(tempDir))
			require.NoError(t, err)

			brickA, found := index.FindBrickByID("arduino:brick-a")
			require.True(t, found)
			assert.Equal(t, tt.wantBrickAModel, brickA.ModelName)

			brickB, found := index.FindBrickByID("arduino:brick-b")
			require.True(t, found)
			assert.Equal(t, tt.wantBrickBModel, brickB.ModelName)
		})
	}
}
