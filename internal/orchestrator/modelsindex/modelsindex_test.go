// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package modelsindex

import (
	"context"
	"fmt"
	"os"
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
		modelsIndex, err := Load(platform.GetPlatform(nil), paths.New("testdata"), paths.New("testdata/models"), nil, "", config.Configuration{})
		require.NoError(t, err)
		require.NotNil(t, modelsIndex)
		models := modelsIndex.loadDryModels()
		assert.Len(t, models, 4, "Expected 4 models to be parsed")
	})

	t.Run("at least one model folders must be provided", func(t *testing.T) {
		_, err := Load(platform.GetPlatform(nil), nil, nil, nil, "", config.Configuration{})
		require.Error(t, err)
	})

	t.Run("custom models folder is optional", func(t *testing.T) {
		modelsIndex, err := Load(platform.GetPlatform(nil), paths.New("testdata"), nil, nil, "", config.Configuration{})
		require.NoError(t, err)
		require.Len(t, modelsIndex.loadDryModels(), 3)
	})

	t.Run("custom models folder can be empty", func(t *testing.T) {
		modelsIndex, err := Load(platform.GetPlatform(nil), nil, paths.New(t.TempDir()), nil, "", config.Configuration{})
		require.NoError(t, err)
		require.Len(t, modelsIndex.loadDryModels(), 0)
	})

	t.Run("it loads nested custom models correctly", func(t *testing.T) {
		modelsIndex, err := Load(platform.GetPlatform(nil), nil, paths.New("testdata/with-nested-models"), nil, "", config.Configuration{})
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
			modelsIndex, err := Load(platform.GetPlatform(nil), paths.New("testdata"), nil, nil, "", config.Configuration{})
			require.NoError(t, err)

			models := modelsIndex.loadDryModels()
			assert.Len(t, models, 3, "all models")
		})

		t.Run("foo-board", func(t *testing.T) {
			platform := platform.Platform{BoardName: "foo-board"}
			modelsIndex, err := Load(platform, paths.New("testdata"), nil, nil, "", config.Configuration{})
			require.NoError(t, err)

			models := modelsIndex.loadDryModels()
			assert.Len(t, models, 3, "all models")
		})

		t.Run("other board", func(t *testing.T) {
			platform := platform.Platform{BoardName: "some-other-board"}
			modelsIndex, err := Load(platform, paths.New("testdata"), nil, nil, "", config.Configuration{})
			require.NoError(t, err)

			models := modelsIndex.loadDryModels()
			assert.Len(t, models, 2, "no model another-model-id")

		})
	})

	t.Run("it gets a preloaded model by ID", func(t *testing.T) {
		modelsIndex, err := Load(platform.GetPlatform(nil), paths.New("testdata"), paths.New("testdata/models"), nil, "", config.Configuration{})
		require.NoError(t, err)
		model, err := modelsIndex.GetModelByID(t.Context(), "not-existing-model")
		require.NoError(t, err)
		assert.Nil(t, model)

		model, err = modelsIndex.GetModelByID(t.Context(), "face-detection")
		require.NoError(t, err)
		require.NotNil(t, model)
		assert.Equal(t, &AIModel{
			ID:                "face-detection",
			Name:              "Lightweight-Face-Detection",
			ModuleDescription: "Face bounding box detection. This model is trained on the WIDER FACE dataset and can detect faces in images.",
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
		modelsIndex, err := Load(platform.GetPlatform(nil), paths.New("testdata"), paths.New("testdata/models"), nil, "", config.Configuration{})
		require.NoError(t, err)

		eimodel, err := modelsIndex.GetModelByID(t.Context(), "my-model-id")
		require.NoError(t, err)
		require.NotNil(t, eimodel)

		assert.Equal(t, &AIModel{
			ID:                "my-model-id",
			Name:              "my custom model from edge impulse",
			ModuleDescription: "A small and accurate model for detecting bounding boxes for faces in images.",
			Bricks:            []BrickConfig{{ID: "object-detection", ModelConfiguration: map[string]string{"AN_ENV_VARIABLE": "/my/env7variable"}}},
			Metadata: map[string]string{
				"a-bool-metadata":   "true",
				"a-int-metadata":    "1",
				"a-string-metadata": "a-string-value",
			},
			ModelFolderPath: paths.New(f.Must(filepath.Abs("testdata/models/my-custom-model"))),
			Installed:       true,
		}, eimodel)
	})

	t.Run("it fails if model-list.yaml does not exist", func(t *testing.T) {
		nonExistentPath := paths.New("nonexistentdir")
		modelsIndex, err := Load(platform.GetPlatform(nil), nonExistentPath, nil, nil, "", config.Configuration{})
		assert.Error(t, err)
		assert.Nil(t, modelsIndex)
	})

	t.Run("it gets models by a brick", func(t *testing.T) {
		modelsIndex, err := Load(platform.GetPlatform(nil), paths.New("testdata"), paths.New("testdata/models"), nil, "", config.Configuration{})
		require.NoError(t, err)

		model := modelsIndex.GetModelsByBrick(t.Context(), "not-existing-brick")
		assert.Nil(t, model)

		model = modelsIndex.GetModelsByBrick(t.Context(), "arduino:object_detection")
		assert.Len(t, model, 1)
		assert.Equal(t, "face-detection", model[0].ID)
	})

	t.Run("it gets models by bricks", func(t *testing.T) {
		modelsIndex, err := Load(platform.GetPlatform(nil), paths.New("testdata"), paths.New("testdata/models"), nil, "", config.Configuration{})
		require.NoError(t, err)

		models := modelsIndex.GetModelsByBricks(t.Context(), []string{"arduino:non_existing"})
		assert.Len(t, models, 0)
		assert.Nil(t, models)

		models = modelsIndex.GetModelsByBricks(t.Context(), []string{"arduino:video_object_detection"})
		assert.Len(t, models, 2)
		assert.Equal(t, "face-detection", models[0].ID)
		assert.Equal(t, "yolox-object-detection", models[1].ID)

		models = modelsIndex.GetModelsByBricks(t.Context(), []string{"arduino:object_detection", "arduino:video_object_detection"})
		assert.Len(t, models, 2)
		assert.Equal(t, "face-detection", models[0].ID)
		assert.Equal(t, "yolox-object-detection", models[1].ID)
	})
}

// fakeArtifact is a file to materialise in the models dir.
type fakeArtifact struct {
	path string // relative to modelsDir
	size int
}

// fakeManifest is a *downloaded.json to drop next to the artifacts.
type fakeManifest struct {
	dir     string // relative to modelsDir
	name    string
	modelID string
	files   map[string]int64 // path (relative to dir) -> declared size
}

func writeFakeArtifact(t *testing.T, path string, size int) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, make([]byte, size), 0o600))
}

func writeFakeManifest(t *testing.T, dir, name, modelID string, files map[string]int64) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	b := fmt.Appendf(nil, `{"version":1,"model_id":%q,"files":[`, modelID)
	first := true
	for p, sz := range files {
		if !first {
			b = append(b, ',')
		}
		first = false
		b = fmt.Appendf(b, `{"path":%q,"size":%d}`, p, sz)
	}
	b = append(b, "]}"...)
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), b, 0o600))
}

func TestInstallStatus(t *testing.T) {
	tests := []struct {
		name          string
		modelID       string
		artifacts     []fakeArtifact
		manifests     []fakeManifest
		wantInstalled bool
		wantSize      uint64 // 0 means: don't assert size (e.g. fallback to metadata)
	}{
		{
			name:          "pre-loaded model is installed from metadata",
			modelID:       "piper-tts-en",
			wantInstalled: true,
			wantSize:      46 * 1024 * 1024,
		},
		{
			name:          "no manifest means not installed",
			modelID:       "ei:efficientnet-b4",
			wantInstalled: false,
		},
		{
			name:    "valid manifest marks model installed with manifest size",
			modelID: "ei:efficientnet-b4",
			artifacts: []fakeArtifact{
				{path: "edge-impulse/efficientnet-b4-qnn.eim", size: 4096},
			},
			manifests: []fakeManifest{{
				dir: "edge-impulse", name: "efficientnet-b4-qnn.eim.downloaded.json",
				modelID: "ei:efficientnet-b4",
				files:   map[string]int64{"efficientnet-b4-qnn.eim": 4096},
			}},
			wantInstalled: true,
			wantSize:      4096,
		},
		{
			name:    "manifest with size mismatch is dropped",
			modelID: "ei:efficientnet-b4",
			artifacts: []fakeArtifact{
				{path: "edge-impulse/efficientnet-b4-qnn.eim", size: 100},
			},
			manifests: []fakeManifest{{
				dir: "edge-impulse", name: "efficientnet-b4-qnn.eim.downloaded.json",
				modelID: "ei:efficientnet-b4",
				files:   map[string]int64{"efficientnet-b4-qnn.eim": 4096},
			}},
			wantInstalled: false,
		},
		{
			name:    "manifests of other models do not leak",
			modelID: "ei:efficientnet-b4",
			artifacts: []fakeArtifact{
				{path: "edge-impulse/intruder.eim", size: 9999},
			},
			manifests: []fakeManifest{{
				dir: "edge-impulse", name: "intruder.eim.downloaded.json",
				modelID: "ei:some-other-model",
				files:   map[string]int64{"intruder.eim": 9999},
			}},
			wantInstalled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelsDir := paths.New(t.TempDir())
			for _, a := range tt.artifacts {
				writeFakeArtifact(t, filepath.Join(modelsDir.String(), a.path), a.size)
			}
			for _, m := range tt.manifests {
				writeFakeManifest(t, filepath.Join(modelsDir.String(), m.dir), m.name, m.modelID, m.files)
			}

			idx, err := Load(
				platform.Platform{BoardName: "ventunoq"},
				paths.New("testdata/with-handlers"),
				modelsDir, nil, "", config.Configuration{},
			)
			require.NoError(t, err)

			model, err := idx.GetModelByID(context.Background(), tt.modelID)
			require.NoError(t, err)
			require.NotNil(t, model)
			assert.Equal(t, tt.wantInstalled, model.Installed)
			if tt.wantSize > 0 {
				assert.Equal(t, tt.wantSize, model.Size)
			}
		})
	}
}

func TestGetModelsPopulatesEveryModel(t *testing.T) {
	modelsDir := paths.New(t.TempDir())
	eiDir := modelsDir.Join("edge-impulse").String()
	writeFakeArtifact(t, filepath.Join(eiDir, "efficientnet-b4-qnn.eim"), 2048)
	writeFakeManifest(t, eiDir, "efficientnet-b4-qnn.eim.downloaded.json", "ei:efficientnet-b4",
		map[string]int64{"efficientnet-b4-qnn.eim": 2048})

	idx, err := Load(
		platform.Platform{BoardName: "ventunoq"},
		paths.New("testdata/with-handlers"),
		modelsDir, nil, "", config.Configuration{},
	)
	require.NoError(t, err)

	byID := map[string]AIModel{}
	for _, m := range idx.GetModels(context.Background()) {
		byID[m.ID] = m
	}

	assert.True(t, byID["piper-tts-en"].Installed)
	assert.Equal(t, uint64(46*1024*1024), byID["piper-tts-en"].Size)

	assert.True(t, byID["ei:efficientnet-b4"].Installed)
	assert.Equal(t, uint64(2048), byID["ei:efficientnet-b4"].Size)
}
