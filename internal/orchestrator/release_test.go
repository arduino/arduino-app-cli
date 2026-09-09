// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/types"
	yaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
)

func TestWriteReleaseArchive(t *testing.T) {
	stagingDir := paths.New(t.TempDir())
	releaseDir := stagingDir.Join("my-app-1.0.0-unoq")

	write := func(mode os.FileMode, content string, part ...string) {
		t.Helper()
		file := releaseDir.Join(part...)
		require.NoError(t, file.Parent().MkdirAll())
		require.NoError(t, file.WriteFile([]byte(content)))
		require.NoError(t, os.Chmod(file.String(), mode))
	}
	write(0o644, "schema: 1\n", ReleaseManifestFileName)
	write(0o644, "name: my-app\n", "src", "app.yaml")
	write(0o755, "#!/bin/sh\n", "src", "python", "run.sh")
	write(0o644, "junk", "src", "__pycache__", "app.pyc")
	write(0o644, "junk", "src", ".cache", "leftover")
	// A package of the venv may well have a data folder, which must ship.
	write(0o644, "weights", "prebuild", ".venv", "lib", "pkg", "data", "weights.bin")

	// Never walked into, and never followed: the target is archived as it is.
	venvLink := releaseDir.Join("prebuild", ".venv", "bin", "python")
	require.NoError(t, venvLink.Parent().MkdirAll())
	require.NoError(t, os.Symlink("../../../../usr/bin/python3", venvLink.String()))

	archivePath := paths.New(t.TempDir()).Join("my-app-1.0.0-unoq" + ReleaseArchiveExt)
	require.NoError(t, writeReleaseArchive(releaseDir, archivePath))

	headers := readArchiveHeaders(t, archivePath)
	names := make([]string, 0, len(headers))
	for _, header := range headers {
		names = append(names, header.Name)
	}

	// The reader must get the release facts from the first block, so the manifest comes
	// right after the folder the archive is rooted at.
	require.Greater(t, len(names), 2)
	assert.Equal(t, "my-app-1.0.0-unoq", names[0])
	assert.Equal(t, "my-app-1.0.0-unoq/release.yaml", names[1])

	assert.Contains(t, names, "my-app-1.0.0-unoq/prebuild/.venv/lib/pkg/data/weights.bin")
	for _, name := range names {
		assert.NotContains(t, name, "__pycache__")
		assert.NotContains(t, name, ".cache/")
	}

	byName := make(map[string]*tar.Header, len(headers))
	for _, header := range headers {
		byName[header.Name] = header
		// The install decides who owns the files.
		assert.Zero(t, header.Uid, header.Name)
		assert.Zero(t, header.Gid, header.Name)
		assert.Empty(t, header.Uname, header.Name)
		assert.Empty(t, header.Gname, header.Name)
	}

	sketchRunner := byName["my-app-1.0.0-unoq/src/python/run.sh"]
	require.NotNil(t, sketchRunner)
	assert.Equal(t, os.FileMode(0o755), os.FileMode(sketchRunner.Mode).Perm())

	link := byName["my-app-1.0.0-unoq/prebuild/.venv/bin/python"]
	require.NotNil(t, link)
	assert.Equal(t, byte(tar.TypeSymlink), link.Typeflag)
	assert.Equal(t, "../../../../usr/bin/python3", link.Linkname)
}

func TestStageReleaseIndexes(t *testing.T) {
	t.Setenv("DOCKER_REGISTRY_BASE", "build.example/")
	cfg := setTestOrchestratorConfig(t)

	// piper-tts-en ships with the board image, ei:efficientnet-b4 is downloaded on install.
	require.NoError(t, cfg.AssetDir().Join("models-list.yaml").WriteFile([]byte(`models:
  - piper-tts-en:
      name: "Piper TTS (English)"
      deployment:
        handler: "ai-hub-handler"
        pre-loaded: true
  - "ei:efficientnet-b4":
      name: "EfficientNet-B4"
      deployment:
        handler: "ei-handler"
`)))
	require.NoError(t, cfg.AssetDir().Join("models-handlers.yaml").WriteFile([]byte(`listing:
  image: ${DOCKER_REGISTRY_BASE}models-downloader:listing
  volumes:
    - ${MODELS_PATH}:/models
handlers:
  - ai-hub-handler:
      description: "Handler for models from AI Hub"
      image: ${DOCKER_REGISTRY_BASE}models-downloader:ai-hub
      volumes:
        - ${MODELS_PATH}:/models
  - ei-handler:
      description: "Handler for models from Edge Impulse"
      image: ${DOCKER_REGISTRY_BASE}models-downloader:ei
      volumes:
        - ${MODELS_PATH}/${models_repository}:/models
`)))

	// No docker client: the listing does not run, so every model is the declared one.
	modelsIndex, err := modelsindex.Load(unoQPlatform, cfg.AssetDir(), cfg.ModelsDir(), cfg.CustomModelsDir(), nil, cfg)
	require.NoError(t, err)

	localBrick := bricksindex.Brick{ID: "local:my_brick"}
	bricksIndex := &bricksindex.BricksIndex{
		BuiltInBricks: []bricksindex.Brick{{ID: "arduino:tts"}, {ID: "arduino:image_classification"}},
		AppBricks:     []bricksindex.Brick{localBrick},
	}
	appToBuild := app.ArduinoApp{
		Name:        "my-app",
		LocalBricks: []bricksindex.Brick{localBrick},
		Descriptor: app.AppDescriptor{Bricks: []app.Brick{
			{ID: localBrick.ID},
			{ID: "arduino:tts", Model: "piper-tts-en"},
			{ID: "arduino:image_classification", Model: "ei:efficientnet-b4"},
		}},
	}

	prebuildDir := paths.New(t.TempDir())
	require.NoError(t, stageReleaseIndexes(context.Background(), prebuildDir, appToBuild, bricksIndex, modelsIndex, cfg, types.Mapping{}))

	content, err := prebuildDir.Join("bricks-list.yaml").ReadFile()
	require.NoError(t, err)
	var bricks bricksindex.YamlBricksIndex
	require.NoError(t, yaml.Unmarshal(content, &bricks))
	// The brick the app brings along ships in src, so the index does not state it.
	assert.Equal(t, []string{"arduino:tts", "arduino:image_classification"},
		[]string{bricks.Bricks[0].ID, bricks.Bricks[1].ID})
	assert.Len(t, bricks.Bricks, 2)

	// The frozen index is read back the way a board reads its own.
	frozen, err := modelsindex.Load(unoQPlatform, prebuildDir, cfg.ModelsDir(), cfg.CustomModelsDir(), nil, cfg)
	require.NoError(t, err)
	assert.True(t, frozen.IsKnown("ei:efficientnet-b4"))
	assert.False(t, frozen.IsKnown("piper-tts-en"), "a built-in model ships with the board image")

	handler, found := frozen.Handlers.GetHandlerByID("ei-handler")
	require.True(t, found)
	// The image is the one the build resolved, not a reference the board answers.
	assert.Equal(t, "build.example/models-downloader:ei", handler.Image)
	_, found = frozen.Handlers.GetHandlerByID("ai-hub-handler")
	assert.False(t, found, "no model of the release names it")
}

func TestReleaseArchivePath(t *testing.T) {
	existing := paths.New(t.TempDir()).Join("taken" + ReleaseArchiveExt)
	require.NoError(t, existing.WriteFile(nil))
	outputDir := paths.New(t.TempDir())

	t.Run("the default is the release name in the current dir", func(t *testing.T) {
		archivePath, err := releaseArchivePath("my-app-1.0.0-unoq", BuildReleaseRequest{})
		require.NoError(t, err)
		cwd, err := os.Getwd()
		require.NoError(t, err)
		assert.Equal(t, paths.New(cwd, "my-app-1.0.0-unoq"+ReleaseArchiveExt).String(), archivePath.String())
	})

	t.Run("an output dir holds the release name", func(t *testing.T) {
		archivePath, err := releaseArchivePath("my-app-1.0.0-unoq", BuildReleaseRequest{Output: outputDir})
		require.NoError(t, err)
		assert.Equal(t, outputDir.Join("my-app-1.0.0-unoq"+ReleaseArchiveExt).String(), archivePath.String())
	})

	t.Run("an output file is the archive", func(t *testing.T) {
		wanted := outputDir.Join("named.arduinoapp")
		archivePath, err := releaseArchivePath("my-app-1.0.0-unoq", BuildReleaseRequest{Output: wanted})
		require.NoError(t, err)
		assert.Equal(t, wanted.String(), archivePath.String())
	})

	t.Run("an existing archive is not overwritten", func(t *testing.T) {
		_, err := releaseArchivePath("my-app-1.0.0-unoq", BuildReleaseRequest{Output: existing})
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrBadRequest))
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("overwrite takes an existing archive", func(t *testing.T) {
		archivePath, err := releaseArchivePath("my-app-1.0.0-unoq", BuildReleaseRequest{Output: existing, Overwrite: true})
		require.NoError(t, err)
		assert.Equal(t, existing.String(), archivePath.String())
	})
}

func readArchiveHeaders(t *testing.T, archivePath *paths.Path) []*tar.Header {
	t.Helper()

	file, err := archivePath.Open()
	require.NoError(t, err)
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	require.NoError(t, err)
	defer gzipReader.Close()

	var headers []*tar.Header
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return headers
		}
		require.NoError(t, err)
		require.False(t, strings.HasPrefix(header.Name, "/"))
		headers = append(headers, header)
	}
}
