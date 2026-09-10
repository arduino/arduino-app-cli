// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/dockerhelper"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
)

func TestExtractImagesFromCompose(t *testing.T) {
	oldPrefixes := dockerhelper.ImagePrefixes
	dockerhelper.ImagePrefixes = []string{"ghcr.io/bcmi-labs/", "public.ecr.aws/arduino/", "ghcr.io/arduino/", "influxdb"}
	defer func() { dockerhelper.ImagePrefixes = oldPrefixes }()

	tests := []struct {
		name           string
		composePath    *paths.Path
		expectedImages []string
		wantErr        bool
	}{
		{
			name:           "valid compose with supported images",
			composePath:    paths.New("testdata", "composes", "service_compose_valid.yaml"),
			expectedImages: []string{"ghcr.io/arduino/app-bricks/ollama-models-runner:dev-next"},
			wantErr:        false,
		},
		{
			name:        "invalid compose",
			composePath: paths.New("testdata", "composes", "service_compose_invalid.yaml"),
			wantErr:     true,
		},
		{
			name:           "no matching prefixes",
			composePath:    paths.New("testdata", "composes", "service_compose_no_prefix_match.yaml"),
			expectedImages: nil,
			wantErr:        false,
		},
		{
			name:        "multiple services with mixed prefixes",
			composePath: paths.New("testdata", "composes", "service_compose_multiple.yaml"),
			expectedImages: []string{
				"ghcr.io/arduino/app-bricks/genie-models-runner:dev-next",
				"ghcr.io/arduino/app-bricks/audio-analytics-models-runner:dev-next",
			},
			wantErr: false,
		},
		{
			name:        "file not found",
			composePath: paths.New("testdata", "composes", "does_not_exist.yaml"),
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := extractImagesFromCompose(tt.composePath)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, tt.expectedImages, got)
			}
		})
	}
}

func TestGetAllSupportedBrickImages(t *testing.T) {
	genieComposePath := paths.New("testdata", "composes", "service_compose_genie.yaml")
	brickComposePath := paths.New("testdata", "composes", "service_compose_valid.yaml")

	services := &servicesindex.ServicesIndex{
		Services: []servicesindex.Service{
			{ServiceID: "arduino:genie", ComposeFile: genieComposePath},
		},
	}

	tests := []struct {
		name           string
		builtIn        []bricksindex.Brick
		expectedImages []string
	}{
		{
			name: "brick without compose but with requires_services pulls service image",
			builtIn: []bricksindex.Brick{
				{
					ID:               "arduino:vlm",
					RequiresServices: bricksindex.RequiresServices{{ID: "arduino:genie"}},
				},
			},
			expectedImages: []string{"ghcr.io/arduino/app-bricks/genie-service:1.0.0"},
		},
		{
			name: "brick with compose and no requires_services",
			builtIn: []bricksindex.Brick{
				{ID: "arduino:runner", ComposeFile: brickComposePath},
			},
			expectedImages: []string{"ghcr.io/arduino/app-bricks/ollama-models-runner:dev-next"},
		},
		{
			name: "brick with compose and requires_services returns both images deduped",
			builtIn: []bricksindex.Brick{
				{
					ID:               "arduino:runner",
					ComposeFile:      brickComposePath,
					RequiresServices: bricksindex.RequiresServices{{ID: "arduino:genie"}},
				},
				{
					ID:               "arduino:vlm",
					RequiresServices: bricksindex.RequiresServices{{ID: "arduino:genie"}},
				},
			},
			expectedImages: []string{
				"ghcr.io/arduino/app-bricks/ollama-models-runner:dev-next",
				"ghcr.io/arduino/app-bricks/genie-service:1.0.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bIndex := &bricksindex.BricksIndex{BuiltInBricks: tt.builtIn}
			got, err := getAllSupportedBrickImages(bIndex, services)
			require.NoError(t, err)
			require.ElementsMatch(t, tt.expectedImages, got)
		})
	}
}
