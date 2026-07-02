// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package modelsindex

import (
	"slices"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

func TestParseDownloadHandlerLine(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		expected    StreamMessage
		expectEvent int
	}{
		{
			name:        "start event",
			line:        `{"event":"start","description":"downloading"}`,
			expectEvent: 1,
			expected: StreamMessage{
				data: "downloading",
			},
		},
		{
			name:        "update event",
			line:        `{"event":"update","current":64,"total":128,"unit":"bytes","percentage":"50%"}`,
			expectEvent: 1,
			expected: StreamMessage{
				progress: new(Progress{Total: 128, Current: 64, Progress: 50}),
			},
		},
		{
			name:        "complete event with artifacts",
			line:        `{"event":"complete","description":"download complete","artifacts":["model.eim","meta.json"]}`,
			expectEvent: 1,
			expected: StreamMessage{
				done: "download complete",
			},
		},
		{
			name:        "error event",
			line:        `{"event":"error","description":"network failure"}`,
			expectEvent: 1,
			expected: StreamMessage{
				err: "network failure",
			},
		},
		{
			name:        "unknown event maps to info",
			line:        `{"event":"something-else","description":"note"}`,
			expectEvent: 0,
			expected:    StreamMessage{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []StreamMessage

			parseDownloadHandlerLine(tt.line, func(e StreamMessage) {
				got = append(got, e)
			})

			require.Len(t, got, tt.expectEvent)
			if tt.expectEvent > 0 {
				assert.Equal(t, tt.expected, got[0])
			}
		})
	}

	t.Run("invalid json does not publish", func(t *testing.T) {
		called := false

		parseDownloadHandlerLine("not-json", func(StreamMessage) {
			called = true
		})

		assert.False(t, called)
	})
}

func TestResolveVars(t *testing.T) {
	t.Run("substitutes a known variable", func(t *testing.T) {
		got := ResolveVars("${FOO}/bar", map[string]string{"FOO": "/opt"})
		assert.Equal(t, "/opt/bar", got)
	})

	t.Run("substitutes multiple variables", func(t *testing.T) {
		got := ResolveVars("${A}:${B}", map[string]string{"A": "hello", "B": "world"})
		assert.Equal(t, "hello:world", got)
	})

	t.Run("substitutes a variable used multiple times", func(t *testing.T) {
		got := ResolveVars("${X}/${X}", map[string]string{"X": "val"})
		assert.Equal(t, "val/val", got)
	})

	t.Run("unknown variable resolves to empty string", func(t *testing.T) {
		got := ResolveVars("${UNSET}/suffix", map[string]string{})
		assert.Equal(t, "/suffix", got)
	})

	t.Run("uses inline default when variable is missing", func(t *testing.T) {
		got := ResolveVars("${REG:-ghcr.io/arduino/}image:tag", map[string]string{})
		assert.Equal(t, "ghcr.io/arduino/image:tag", got)
	})

	t.Run("provided value takes precedence over inline default", func(t *testing.T) {
		got := ResolveVars("${REG:-ghcr.io/arduino/}image:tag", map[string]string{"REG": "myregistry.io/"})
		assert.Equal(t, "myregistry.io/image:tag", got)
	})
}

func TestGetImagesHandlersFromInlineYAML(t *testing.T) {
	tempDir := paths.New(t.TempDir())

	yamlContent := `listing:
  image: test-registry/models-downloader:listing
  volumes:
    - ${MODELS_PATH}:/models
  command: ["/app/list_models.sh"]
handlers:
  - ai-hub-handler:
      description: "Handler for models from AI Hub"
      image: test-registry/models-downloader:ai-hub
      volumes:
        - ${MODELS_PATH}/${models_repository}:/models
      actions:
        - download:
            command: ["/app/ai_hub/ai_hub_model_downloader.sh"]
        - delete:
            command: ["/app/ai_hub/ai_hub_model_remover.sh"]
        - check:
            command: ["/app/ai_hub/ai_hub_model_checker.sh"]
        - info:
            command: ["/app/ai_hub/ai_hub_model_info.sh"]
  - ei-handler:
      description: "Handler for models from Edge Impulse"
      image: test-registry/models-downloader:ei
      volumes:
        - ${MODELS_PATH}/${models_repository}:/models
      actions:
        - download:
            command: ["/app/edge_impulse/ei_model_downloader.sh"]
        - delete:
            command: ["/app/edge_impulse/ei_model_remover.sh"]
        - check:
            command: ["/app/edge_impulse/ei_model_checker.sh"]
        - info:
            command: ["/app/edge_impulse/ei_model_info.sh"]
  - hf-handler:
      description: "Handler for models from Hugging Face"
      image: test-registry/models-downloader:hf
      volumes:
        - ${MODELS_PATH}/${models_repository}:/models
      actions:
        - download:
            command: ["/app/hugging_face/hf_model_downloader.sh"]
        - delete:
            command: ["/app/hugging_face/hf_model_remover.sh"]
        - check:
            command: ["/app/hugging_face/hf_model_checker.sh"]
        - info:
            command: ["/app/hugging_face/hf_model_info.sh"]
`

	err := tempDir.Join("models-handlers.yaml").WriteFile([]byte(yamlContent))
	require.NoError(t, err)

	customModelsDir := paths.New(t.TempDir()).Join("models")
	handlersIndex, err := loadHandlers(tempDir, customModelsDir, config.Configuration{}, platform.Platform{})
	require.NoError(t, err)
	require.NotNil(t, handlersIndex)

	images := handlersIndex.GetDockerImages()
	slices.Sort(images)
	assert.Equal(t, []string{"test-registry/models-downloader:ai-hub", "test-registry/models-downloader:ei", "test-registry/models-downloader:hf", "test-registry/models-downloader:listing"}, images)
}
