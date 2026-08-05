// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"path/filepath"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func TestFilterExamples(t *testing.T) {
	var yamlContent = `
bricks:
- id: arduino:a-supported-brick
  name: A Supported Brick
  description: "This is a good brick that does good things."
  ports:
     - 6000
- id: arduino:unsupported-brick
  name: Unsuported Brick
  description: "This is another good brick that does even better things."
  supported_boards:
     - foo-board
`
	var exampleJsonContent = `
{
  "core-and-foundational": [
    {
      "category": "09-computer-vision",
      "examples": [
        {
          "id": "examples:core-and-foundational/09-computer-vision/01-simple-object-detection"
        },
        {
          "id": "examples:core-and-foundational/09-computer-vision/04-object-detection-with-ui"
        }
      ]
    }
  ],
  "bricks": [
    {
      "brick": "arduino:a-supported-brick",
      "examples": [
      ]
    },
    {
      "brick": "arduino:unsupported-brick",
      "examples": [
      ]
    }
  ]
}
`
	assetDir := paths.TempDir()
	err := assetDir.Join("bricks-list.yaml").WriteFile([]byte(yamlContent))
	require.NoError(t, err)
	err = assetDir.Join("examples").MkdirAll()
	require.NoError(t, err)
	err = assetDir.Join("examples").Join("examples.json").WriteFile([]byte(exampleJsonContent))
	require.NoError(t, err)

	brickIndex, err := bricksindex.Load(unoQPlatform, assetDir)
	if err != nil {
		t.Fatalf("failed to load bricks index: %v", err)
	}
	t.Setenv("ARDUINO_APP_CLI__DATA_DIR", filepath.Join(assetDir.String()))

	cfg, err := config.NewFromEnv()
	idProvider := appid.NewAppProvider(cfg, unoQPlatform)

	testCases := []struct {
		name                string
		expectedBrickLength int
	}{
		{
			name:                "Filter unsupported bricks",
			expectedBrickLength: 1,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			examples, err := GetExamples(cfg, brickIndex, idProvider)
			require.NoError(t, err)
			require.Equal(t, tc.expectedBrickLength, len(examples.Bricks))
		})
	}
}
