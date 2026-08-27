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
			name:        "complete event ignores the file list",
			line:        `{"event":"complete","description":"download complete","artifacts":["model.eim","meta.json"]}`,
			expectEvent: 1,
			expected: StreamMessage{
				done: "download complete",
			},
		},
		{
			name:        "info event reports the handler's line and ignores the file list",
			line:        `{"event":"info","description":"Downloaded to: /models/repo","artifacts":["/models/repo/m.gguf"]}`,
			expectEvent: 1,
			expected: StreamMessage{
				data: "Downloaded to: /models/repo",
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

func testHandlersIndex() *HandlersIndex {
	return &HandlersIndex{
		handlers:  map[string]ModelHandler{"hf-handler": {ID: "hf-handler"}},
		configEnv: map[string]string{"BOARD_NAME": "unoq"},
	}
}

func TestUserConfiguredModel(t *testing.T) {
	inputs := map[string]string{
		"models_repository": "llamacpp",
		"model_directory":   "unsloth/Qwen3.5-0.8B-GGUF",
		"model_url":         "https://huggingface.co/unsloth/Qwen3.5-0.8B-GGUF/blob/f4db1b3/Qwen3.5-0.8B-Q4_0.gguf",
	}
	entry := func(mutate func(*handlerModelEntry)) handlerModelEntry {
		// An ad-hoc id is qualified by the repository directory the file landed in, so it
		// cannot collide with a same-named GGUF from another owner.
		e := handlerModelEntry{
			ID:          "llamacpp:unsloth/Qwen3.5-0.8B-GGUF/Qwen3.5-0.8B-Q4_0",
			Name:        "unsloth/Qwen3.5-0.8B-GGUF/Qwen3.5-0.8B-Q4_0",
			Handler:     "llamacpp",
			ModelOrigin: "user_configured",
			Metadata: &entryMetadata{
				ModelID: "llamacpp:unsloth/Qwen3.5-0.8B-GGUF/Qwen3.5-0.8B-Q4_0",
				Handler: "hf-handler",
				Inputs:  inputs,
			},
		}
		if mutate != nil {
			mutate(&e)
		}
		return e
	}

	t.Run("appends a model no models-list.yaml entry declares", func(t *testing.T) {
		model, ok := testHandlersIndex().userDownloadModel(entry(nil))
		require.True(t, ok)
		assert.Equal(t, "llamacpp:unsloth/Qwen3.5-0.8B-GGUF/Qwen3.5-0.8B-Q4_0", model.ID)
		assert.Equal(t, "unsloth/Qwen3.5-0.8B-GGUF/Qwen3.5-0.8B-Q4_0", model.Name)
		assert.False(t, model.IsBuiltIn, "a built-in model cannot be deleted")
		assert.Nil(t, model.ModelFolderPath, "the listing's path is a container path")
		require.NotNil(t, model.Deployment)
		// The handler id comes from the record: entry.Handler is a namespace.
		assert.Equal(t, "hf-handler", model.Deployment.Handler)
		assert.Equal(t, inputs, model.Deployment.VariablesForPlatform("unoq"))
		assert.Equal(t, []BrickConfig{{ID: "arduino:llm"}}, model.Bricks)
	})

	// models-downloader <= 0.12.0 writes neither field, so on an older image every
	// undeclared entry falls into one of these two cases and nothing is appended.
	t.Run("skips an entry the handler marks builtin", func(t *testing.T) {
		_, ok := testHandlersIndex().userDownloadModel(entry(func(e *handlerModelEntry) {
			e.ModelOrigin = "builtin"
		}))
		assert.False(t, ok)
	})

	t.Run("skips an entry with no download record", func(t *testing.T) {
		_, ok := testHandlersIndex().userDownloadModel(entry(func(e *handlerModelEntry) {
			e.Metadata = nil
		}))
		assert.False(t, ok)
	})

	t.Run("skips an entry whose record carries no inputs", func(t *testing.T) {
		_, ok := testHandlersIndex().userDownloadModel(entry(func(e *handlerModelEntry) {
			e.Metadata.Inputs = nil
		}))
		assert.False(t, ok)
	})

	// Two quantizations of one repository share a record naming the last downloaded.
	t.Run("skips an entry whose record names another model", func(t *testing.T) {
		_, ok := testHandlersIndex().userDownloadModel(entry(func(e *handlerModelEntry) {
			e.Metadata.ModelID = "llamacpp:unsloth/Qwen3.5-0.8B-GGUF/Qwen3.5-0.8B-Q8_0"
		}))
		assert.False(t, ok)
	})

	t.Run("skips an entry naming a handler the index does not know", func(t *testing.T) {
		_, ok := testHandlersIndex().userDownloadModel(entry(func(e *handlerModelEntry) {
			e.Metadata.Handler = "not-a-handler"
		}))
		assert.False(t, ok)
	})
}

func TestApplyStatusTo(t *testing.T) {
	diskSize, yamlSize := 507.0, 480.0

	t.Run("installed prefers the on-disk size", func(t *testing.T) {
		var model AIModel
		handlerModelEntry{Installed: true, DiskSizeMB: &diskSize, ModelSizeMB: &yamlSize}.applyStat(&model)
		assert.Equal(t, InstalledStatus, model.Status)
		assert.False(t, model.Downloading)
		assert.Equal(t, uint64(507*1024*1024), model.Size)
	})

	t.Run("not installed falls back to the declared size", func(t *testing.T) {
		var model AIModel
		handlerModelEntry{Installed: false, DiskSizeMB: &diskSize, ModelSizeMB: &yamlSize}.applyStat(&model)
		assert.Equal(t, NotInstalledStatus, model.Status)
		assert.Equal(t, uint64(480*1024*1024), model.Size)
	})

	t.Run("downloading is not installed", func(t *testing.T) {
		var model AIModel
		handlerModelEntry{Installed: false, Downloading: true}.applyStat(&model)
		assert.Equal(t, NotInstalledStatus, model.Status)
		assert.True(t, model.Downloading)
		assert.Zero(t, model.Size)
	})
}

func TestParseDownloadHandlerLineNamesTheModel(t *testing.T) {
	t.Run("an info event carrying an id names the model", func(t *testing.T) {
		var got []StreamMessage
		parseDownloadHandlerLine(`{"event":"info","description":"Downloaded to: /models/org/repo","artifacts":["/models/org/repo/m-Q4_0.gguf"],"model_id":"llamacpp:org/repo/m-Q4_0","size_mb":2048}`, func(e StreamMessage) {
			got = append(got, e)
		})

		require.Len(t, got, 1)
		require.NotNil(t, got[0].GetModel())
		assert.Equal(t, "llamacpp:org/repo/m-Q4_0", got[0].GetModel().ID)
		assert.Equal(t, uint64(2048*1024*1024), got[0].GetModel().Size)
	})

	t.Run("an info event without an id names nothing", func(t *testing.T) {
		// A handler too old to report the id, or one whose record could not be written:
		// either way there is no model to answer with.
		var got []StreamMessage
		parseDownloadHandlerLine(`{"event":"info","description":"Downloading","artifacts":["/models/org/repo/m-Q4_0.gguf"]}`, func(e StreamMessage) {
			got = append(got, e)
		})

		require.Len(t, got, 1)
		assert.Nil(t, got[0].GetModel())
	})
}

func TestUserConfiguredModelFromDownloadEvent(t *testing.T) {
	id := "llamacpp:unsloth/SmolLM2-135M-Instruct-GGUF/SmolLM2-135M-Instruct-Q4_K_M"
	model := UserConfiguredModel(DownloadedModel{ID: id, Size: 1024})

	assert.Equal(t, id, model.ID)
	// The whole path survives: it is what the listing reports for the same files, and
	// what models.ini serves the model under, so shortening it here would name the
	// model something no other component knows.
	assert.Equal(t, "unsloth/SmolLM2-135M-Instruct-GGUF/SmolLM2-135M-Instruct-Q4_K_M", model.Name,
		"the name is the id without its framework namespace")
	assert.Equal(t, InstalledStatus, model.Status)
	assert.Equal(t, uint64(1024), model.Size)
	assert.Equal(t, []BrickConfig{{ID: "arduino:llm"}}, model.Bricks)
	assert.False(t, model.IsBuiltIn)
}

func TestUserConfiguredModelMatchesTheListing(t *testing.T) {
	// The same files, described once from a download event and once from a listing entry,
	// must not come out differently named.
	entry := handlerModelEntry{
		ID:          "llamacpp:org/repo/m-Q4_0",
		Name:        "org/repo/m-Q4_0",
		ModelOrigin: "user_configured",
		Installed:   true,
		Metadata:    &entryMetadata{ModelID: "llamacpp:org/repo/m-Q4_0", Handler: "hf-handler", Inputs: map[string]string{"model_url": "llamacpp:org/repo:Q4_0"}},
	}
	listed, ok := testHandlersIndex().userDownloadModel(entry)
	require.True(t, ok)

	fromEvent := UserConfiguredModel(DownloadedModel{ID: entry.ID})
	assert.Equal(t, listed.ID, fromEvent.ID)
	assert.Equal(t, listed.Name, fromEvent.Name)
	assert.Equal(t, listed.Bricks, fromEvent.Bricks)
}

func TestModelNameFromID(t *testing.T) {
	assert.Equal(t, "m-Q4_0", modelNameFromID("llamacpp:m-Q4_0"))
	assert.Equal(t, "bare", modelNameFromID("bare"), "an id with no namespace is its own name")
	// Only the framework namespace is cut. An ad-hoc id carries the repository path it
	// was downloaded from, and every segment of it is part of the name.
	assert.Equal(t, "unsloth/Qwen3-0.6B-GGUF/Qwen3-0.6B-Q4_0",
		modelNameFromID("llamacpp:unsloth/Qwen3-0.6B-GGUF/Qwen3-0.6B-Q4_0"))
}

func TestDeclaredByIDNeedsNoHandler(t *testing.T) {
	// The install route asks this before deciding whether an id names a declared model or
	// a download source, so it must answer from models-list.yaml alone - a nil handlers
	// index and a nil docker client stand in for "no container available".
	idx := &ModelsIndex{
		InternalModels: []AIModel{
			{ID: "llamacpp:Declared-Q4_0", Name: "Declared"},
			// A key holding a slash: nothing forbids one, and it must still be found here
			// rather than being taken for a Hugging Face repository.
			{ID: "vendor/slashed-id", Name: "Slashed"},
		},
	}

	model, ok := idx.DeclaredByID("llamacpp:Declared-Q4_0")
	require.True(t, ok)
	assert.Equal(t, "Declared", model.Name)

	model, ok = idx.DeclaredByID("vendor/slashed-id")
	require.True(t, ok)
	assert.Equal(t, "Slashed", model.Name)

	_, ok = idx.DeclaredByID("unsloth/SmolLM2-135M-Instruct-GGUF")
	assert.False(t, ok, "a repository the catalog does not declare is not a declared model")
}
