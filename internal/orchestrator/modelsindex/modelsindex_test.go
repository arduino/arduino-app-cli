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
		assert.Len(t, models, 4, "Expected 4 models to be parsed")
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
			assert.Len(t, models, 3, "all models")
		})

		t.Run("foo-board", func(t *testing.T) {
			platform := platform.Platform{BoardName: "foo-board"}
			modelsIndex, err := Load(platform, paths.New("testdata"), paths.New(t.TempDir()), nil, nil, config.Configuration{})
			require.NoError(t, err)

			models := modelsIndex.loadDryModels()
			assert.Len(t, models, 3, "all models")
		})

		t.Run("other board", func(t *testing.T) {
			platform := platform.Platform{BoardName: "some-other-board"}
			modelsIndex, err := Load(platform, paths.New("testdata"), paths.New(t.TempDir()), nil, nil, config.Configuration{})
			require.NoError(t, err)

			models := modelsIndex.loadDryModels()
			assert.Len(t, models, 2, "no model another-model-id")

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
			IsInternal:  true,
			Installed:   true,
		}, model)
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
			Installed:       true,
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

		model := modelsIndex.GetModelsByBrick("not-existing-brick")
		assert.Empty(t, model)

		model = modelsIndex.GetModelsByBrick("arduino:object_detection")
		assert.Len(t, model, 1)
		assert.Equal(t, "face-detection", model[0].ID)
	})
}
