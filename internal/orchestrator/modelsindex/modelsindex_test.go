// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package modelsindex

import (
	"path/filepath"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

func TestModelsIndex(t *testing.T) {
	t.Run("it parses a valid model-list.yaml and custom models", func(t *testing.T) {
		modelsIndex, err := Load(platform.GetPlatform(nil), paths.New("testdata"), paths.New("path-not-existing"), paths.New("testdata/custom-models"), nil, config.Configuration{})
		require.NoError(t, err)
		require.NotNil(t, modelsIndex)
		models := modelsIndex.loadDryModels()
		assert.Len(t, models, 7, "Expected 6 models to be parsed")
	})

	t.Run("dir and modelsDir are required", func(t *testing.T) {
		_, err := Load(platform.GetPlatform(nil), nil, nil, nil, nil, config.Configuration{})
		require.Error(t, err)

		_, err = Load(platform.GetPlatform(nil), paths.New("testdata"), nil, nil, nil, config.Configuration{})
		require.Error(t, err)

		_, err = Load(platform.GetPlatform(nil), nil, paths.New(t.TempDir()), nil, nil, config.Configuration{})
		require.Error(t, err)
	})

	t.Run("custom models folder can be empty", func(t *testing.T) {
		dir := paths.New(t.TempDir())
		require.NoError(t, dir.Join("models-list.yaml").WriteFile([]byte("models: []\n")))
		modelsIndex, err := Load(platform.GetPlatform(nil), dir, paths.New(t.TempDir()), nil, nil, config.Configuration{})
		require.NoError(t, err)
		require.Len(t, modelsIndex.loadDryModels(), 0)
	})

	t.Run("it loads nested custom models correctly", func(t *testing.T) {
		dir := paths.New(t.TempDir())
		require.NoError(t, dir.Join("models-list.yaml").WriteFile([]byte("models: []\n")))
		modelsIndex, err := Load(platform.GetPlatform(nil), dir, paths.New("path-not-existing"), paths.New("testdata/with-nested-models"), nil, config.Configuration{})
		assert.NoError(t, err)
		assert.NotEmpty(t, modelsIndex)
		assert.Len(t, modelsIndex.loadDryModels(), 2)

		got := modelsIndex.loadDryModels()

		assert.Equal(t, f.Must(filepath.Abs("testdata/with-nested-models/nested/nested-model")), got[1].ModelFolderPath.String())
		assert.Equal(t, "my-nested-model-id", got[1].ID)

		assert.Equal(t, f.Must(filepath.Abs("testdata/with-nested-models/another-model")), got[0].ModelFolderPath.String())
		assert.Equal(t, "another-model-id", got[0].ID)
	})

	t.Run("it filter model for supported boards", func(t *testing.T) {
		t.Run("app", func(t *testing.T) {
			modelsIndex, err := Load(platform.GetPlatform(nil), paths.New("testdata"), paths.New(t.TempDir()), nil, nil, config.Configuration{})
			require.NoError(t, err)

			models := modelsIndex.loadDryModels()
			assert.Len(t, models, 6, "all models")
		})

		t.Run("foo-board", func(t *testing.T) {
			platform := platform.Platform{BoardName: "foo-board"}
			modelsIndex, err := Load(platform, paths.New("testdata"), paths.New(t.TempDir()), nil, nil, config.Configuration{})
			require.NoError(t, err)

			models := modelsIndex.loadDryModels()
			assert.Len(t, models, 6, "all models")
		})

		t.Run("other board", func(t *testing.T) {
			platform := platform.Platform{BoardName: "some-other-board"}
			modelsIndex, err := Load(platform, paths.New("testdata"), paths.New(t.TempDir()), nil, nil, config.Configuration{})
			require.NoError(t, err)

			models := modelsIndex.loadDryModels()
			assert.Len(t, models, 5, "no model another-model-id")

		})
	})

	t.Run("it gets a preloaded model by ID", func(t *testing.T) {
		modelsIndex, err := Load(platform.GetPlatform(nil), paths.New("testdata"), paths.New("testdata/custom-models"), nil, nil, config.Configuration{})
		require.NoError(t, err)
		model, err := modelsIndex.GetModelByID(t.Context(), "not-existing-model")
		require.NoError(t, err)
		assert.Nil(t, model)

		model, err = modelsIndex.GetModelByID(t.Context(), "face-detection")
		require.NoError(t, err)
		require.NotNil(t, model)
		assert.Equal(t, &AIModel{
			ID:          "face-detection",
			Name:        "Lightweight-Face-Detection",
			Description: "Face bounding box detection. This model is trained on the WIDER FACE dataset and can detect faces in images.",
			Bricks: []BrickConfig{
				{ID: "arduino:object_detection", ModelConfiguration: map[string]string{"EI_OBJ_DETECTION_MODEL": "/models/ootb/ei/lw-face-det.eim"}},
				{ID: "arduino:video_object_detection", ModelConfiguration: map[string]string{"EI_V_OBJ_DETECTION_MODEL": "/models/ootb/ei/video-face-det.eim"}},
			},
			Metadata: map[string]string{
				"source":           "qualcomm-ai-hub",
				"ei-gpu-mode":      "false",
				"source-model-id":  "face-det-lite",
				"source-model-url": "https://aihub.qualcomm.com/models/face_det_lite",
			},
			ModelLabels: []string{"face"},
			Runner:      "brick",
			IsBuiltIn:   true,
			Origin:      CuratedOrigin,
			Status:      InstalledStatus,
		}, model)

	})

	t.Run("it load builtin model", func(t *testing.T) {
		modelsIndex, err := Load(platform.GetPlatform(nil), paths.New("testdata"), paths.New("testdata/models"), nil, nil, config.Configuration{})
		require.NoError(t, err)

		model, err := modelsIndex.GetModelByID(t.Context(), "a-builtin-model")
		require.NoError(t, err)
		require.NotNil(t, model)
		assert.Equal(t, &AIModel{
			ID: "a-builtin-model",
			Deployment: &ModelDeployment{
				Handler:   "",
				PreLoaded: true,
			},
			IsBuiltIn: true,
			Origin:    CuratedOrigin,
			Status:    InstalledStatus,
		}, model)
		assert.Equal(t, InstalledStatus, model.Status)

		model, err = modelsIndex.GetModelByID(t.Context(), "a-builtin-model-with-handler")
		require.NoError(t, err)
		require.NotNil(t, model)
		assert.Equal(t, &AIModel{
			ID: "a-builtin-model-with-handler",
			Deployment: &ModelDeployment{
				Handler:   "my-handler",
				PreLoaded: true,
			},
			IsBuiltIn: true,
			Origin:    CuratedOrigin,
			Status:    InstalledStatus,
		}, model)
		assert.Equal(t, InstalledStatus, model.Status)
	})

	t.Run("it read the status of the model from the handler listing", func(t *testing.T) {
		t.Run("installed", func(t *testing.T) {
			cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
				return listingWith(`{"id":"a-model-not-preloaded-with-handler","installed":true}`), 0
			})
			modelsIndex, err := Load(platform.GetPlatform(nil), paths.New("testdata"), paths.New("testdata/models"), nil, cli, config.Configuration{})
			require.NoError(t, err)

			model, err := modelsIndex.GetModelByID(t.Context(), "a-model-not-preloaded-with-handler")
			require.NoError(t, err)
			require.NotNil(t, model)
			assert.Equal(t, &AIModel{
				ID: "a-model-not-preloaded-with-handler",
				Deployment: &ModelDeployment{
					Handler:   "my-handler",
					PreLoaded: false,
				},
				IsBuiltIn: false,
				Origin:    CuratedOrigin,
				Status:    InstalledStatus,
			}, model)
		})

		t.Run("not installed: a download is in flight", func(t *testing.T) {
			cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
				return listingWith(`{"id":"a-model-not-preloaded-with-handler","installed":false,"downloading":true}`), 0
			})
			modelsIndex, err := Load(platform.GetPlatform(nil), paths.New("testdata"), paths.New("testdata/models"), nil, cli, config.Configuration{})
			require.NoError(t, err)

			model, err := modelsIndex.GetModelByID(t.Context(), "a-model-not-preloaded-with-handler")
			require.NoError(t, err)
			require.NotNil(t, model)
			assert.Equal(t, &AIModel{
				ID: "a-model-not-preloaded-with-handler",
				Deployment: &ModelDeployment{
					Handler:   "my-handler",
					PreLoaded: false,
				},
				IsBuiltIn:   false,
				Origin:      CuratedOrigin,
				Status:      NotInstalledStatus,
				Downloading: true,
			}, model)
		})

		t.Run("not installed: absent from disk", func(t *testing.T) {
			cli := newFakeDockerClient(func(image string, cmd []string) (string, int) {
				return listingWith(`{"id":"a-model-not-preloaded-with-handler","installed":false}`), 0
			})
			modelsIndex, err := Load(platform.GetPlatform(nil), paths.New("testdata"), paths.New("testdata/models"), nil, cli, config.Configuration{})
			require.NoError(t, err)

			model, err := modelsIndex.GetModelByID(t.Context(), "a-model-not-preloaded-with-handler")
			require.NoError(t, err)
			require.NotNil(t, model)
			assert.Equal(t, NotInstalledStatus, model.Status)
		})
	})

	t.Run("it get custom model by id", func(t *testing.T) {
		modelsIndex, err := Load(platform.GetPlatform(nil), paths.New("testdata"), paths.New("not-existing-path"), paths.New("testdata/custom-models"), nil, config.Configuration{})
		require.NoError(t, err)

		eimodel, err := modelsIndex.GetModelByID(t.Context(), "my-model-id")
		require.NoError(t, err)
		require.NotNil(t, eimodel)

		assert.Equal(t, &AIModel{
			ID:          "my-model-id",
			Name:        "my custom model from edge impulse",
			Description: "A small and accurate model for detecting bounding boxes for faces in images.",
			Bricks:      []BrickConfig{{ID: "object-detection", ModelConfiguration: map[string]string{"AN_ENV_VARIABLE": "/my/env7variable"}}},
			Metadata: map[string]string{
				"a-bool-metadata":   "true",
				"a-int-metadata":    "1",
				"a-string-metadata": "a-string-value",
			},
			ModelFolderPath: paths.New(f.Must(filepath.Abs("testdata/custom-models/my-custom-model"))),
			Status:          InstalledStatus,
			IsBuiltIn:       false, // a custom model is never built-in
			Origin:          EdgeImpulseOrigin,
		}, eimodel)
	})

	t.Run("it fails if model-list.yaml does not exist", func(t *testing.T) {
		nonExistentPath := paths.New("nonexistentdir")
		modelsIndex, err := Load(platform.GetPlatform(nil), nonExistentPath, paths.New(t.TempDir()), nil, nil, config.Configuration{})
		assert.Error(t, err)
		assert.Nil(t, modelsIndex)
	})

	t.Run("it gets models by a brick", func(t *testing.T) {
		modelsIndex, err := Load(platform.GetPlatform(nil), paths.New("testdata"), paths.New("path-not-existing"), paths.New("testdata/custom-models"), nil, config.Configuration{})
		require.NoError(t, err)

		model := modelsIndex.GetModelsByBrick(t.Context(), "not-existing-brick")
		assert.Empty(t, model)

		model = modelsIndex.GetModelsByBrick(t.Context(), "arduino:object_detection")
		assert.Len(t, model, 1)
		assert.Equal(t, "face-detection", model[0].ID)
	})
}

// TestInstalledByDeclaration pins the one predicate a lookup and the install route share.
// They used to test different things: the lookup asked for "pre-loaded", the install route
// for "names no handler". Every pre-loaded entry in models-list.yaml names a handler, so
// the install route sent all of them to the downloader with an empty variable map.
func TestInstalledByDeclaration(t *testing.T) {
	tests := []struct {
		name  string
		model AIModel
		want  bool
	}{
		{
			name:  "a custom model has no deployment at all",
			model: AIModel{ID: "ei-model-990187-1"},
			want:  true,
		},
		{
			name:  "a pre-loaded model still names the handler that built it",
			model: AIModel{ID: "piper-tts-en", Deployment: &ModelDeployment{Handler: "ai-hub-handler", PreLoaded: true}},
			want:  true,
		},
		{
			name:  "a downloadable model is installed by its handler, not its declaration",
			model: AIModel{ID: "ei:efficientnet-b4", Deployment: &ModelDeployment{Handler: "ei-handler"}},
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.model.InstalledByDeclaration())
		})
	}

	t.Run("every pre-loaded entry in the shipped model list is one", func(t *testing.T) {
		// The install route runs no handler for these. A pre-loaded entry that answered
		// false would start the ai-hub or Edge Impulse downloader with no variables, so
		// its models_repository would resolve empty and bind the whole models directory.
		// The copy "task test:internal" downloads, not the one the deb build writes: that
		// one is gitignored, so on CI it is not there.
		dir := paths.New("../../../internal/e2e/daemon/testdata/assets", config.RunnerVersion)
		idx, err := Load(platform.Platform{BoardName: "ventunoq"}, dir, paths.New("not-existing-path"), nil, nil, config.Configuration{})
		require.NoError(t, err)

		var preLoaded int
		for _, model := range idx.InternalModels {
			if model.Deployment == nil || !model.Deployment.PreLoaded {
				continue
			}
			preLoaded++
			assert.True(t, model.InstalledByDeclaration(), "model %q", model.ID)
		}
		assert.NotZero(t, preLoaded, "the model list must still declare pre-loaded models")
	})
}

// TestMatchesID pins the rule that lets one identity travel in two forms. The model is
// what recognizes its own id, so nothing along the way has to decode a caller's string
// and guess what it meant.
func TestMatchesID(t *testing.T) {
	for _, id := range []string{
		"face-detection",
		"llamacpp:Qwen3.5-0.8B-Q4_0",
		"ei-model-901144-1",
		"vendor/slashed-id",
		"llamacpp:ggml-org/SmolVLM-256M-Instruct-GGUF/SmolVLM-256M-Instruct-Q8_0",
	} {
		model := AIModel{ID: id}
		assert.True(t, matchesID(model, id), "the plain id names the model: %q", id)
		assert.True(t, matchesID(model, EncodeID(id)), "so does its encoding: %q", id)
	}

	// Neither a different model nor a wrongly padded encoding is a match.
	model := AIModel{ID: "face-detection"}
	assert.False(t, matchesID(model, "person-classification"))
	assert.False(t, matchesID(model, EncodeID("face-detection")+"="))
	assert.False(t, matchesID(model, ""))
}
