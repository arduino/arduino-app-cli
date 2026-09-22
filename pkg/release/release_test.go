// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package release

import (
	"archive/tar"
	"compress/gzip"
	"testing"
	"time"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTestArchive writes a minimal release archive, entries in the given order, root
// being the release folder every entry is written under.
func writeTestArchive(t *testing.T, root string, entries map[string]string, order []string) *paths.Path {
	t.Helper()
	archivePath := paths.New(t.TempDir()).Join(root + ".arduinoapp")

	file, err := archivePath.Create()
	require.NoError(t, err)
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	require.NoError(t, tarWriter.WriteHeader(&tar.Header{Name: root + "/", Typeflag: tar.TypeDir, Mode: 0o755}))
	for _, name := range order {
		content := entries[name]
		require.NoError(t, tarWriter.WriteHeader(&tar.Header{
			Name: root + "/" + name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(content)),
		}))
		_, err := tarWriter.Write([]byte(content))
		require.NoError(t, err)
	}

	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzipWriter.Close())
	require.NoError(t, file.Close())
	return archivePath
}

func TestReadReleaseInfo(t *testing.T) {
	manifest := "schema: 1\n" +
		"name: my-app\n" +
		"target: unoq\n" +
		"created_at: 2026-09-14T13:45:12Z\n" +
		"notes: hello\n"

	archivePath := writeTestArchive(t, "my-app-1.0.0-unoq", map[string]string{
		"release.yaml": manifest,
		"src/app.yaml": "name: my-app\n",
	}, []string{"release.yaml", "src/app.yaml"})

	info, err := ReadReleaseInfo(archivePath)
	require.NoError(t, err)
	assert.Equal(t, 1, info.Schema)
	assert.Equal(t, "my-app", info.Name)
	assert.Equal(t, "unoq", info.Target)
	assert.True(t, info.CreatedAt.Equal(time.Date(2026, 9, 14, 13, 45, 12, 0, time.UTC)))
	assert.Equal(t, "hello", info.Notes)
}

func TestReadReleaseInfoNotAReleaseArchive(t *testing.T) {
	archivePath := paths.New(t.TempDir()).Join("not-an-archive.arduinoapp")
	require.NoError(t, archivePath.WriteFile([]byte("plain text, no gzip header")))

	_, err := ReadReleaseInfo(archivePath)
	assert.ErrorContains(t, err, "is not a release archive")
}

func TestReadReleaseInfoWrongExtension(t *testing.T) {
	archivePath := writeTestArchive(t, "my-app-1.0.0-unoq", map[string]string{
		"release.yaml": "schema: 1\nname: my-app\ntarget: unoq\n",
	}, []string{"release.yaml"})
	renamed := archivePath.Parent().Join("my-app-1.0.0-unoq.tar.gz")
	require.NoError(t, archivePath.Rename(renamed))

	_, err := ReadReleaseInfo(renamed)
	assert.ErrorContains(t, err, "is not a release archive")
}

func TestReadReleaseInfoNoManifest(t *testing.T) {
	archivePath := writeTestArchive(t, "my-app-1.0.0-unoq", map[string]string{
		"src/app.yaml": "name: my-app\n",
	}, []string{"src/app.yaml"})

	_, err := ReadReleaseInfo(archivePath)
	assert.ErrorContains(t, err, "no release manifest")
}

func TestReadReleaseInfoIncompleteManifest(t *testing.T) {
	archivePath := writeTestArchive(t, "my-app-1.0.0-unoq", map[string]string{
		"release.yaml": "schema: 1\n",
	}, []string{"release.yaml"})

	_, err := ReadReleaseInfo(archivePath)
	assert.ErrorContains(t, err, "states no name or no target")
}

func TestReadReleaseInfoMultipleRoots(t *testing.T) {
	archivePath := paths.New(t.TempDir()).Join("my-app-1.0.0-unoq.arduinoapp")

	file, err := archivePath.Create()
	require.NoError(t, err)
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)

	// Two different top-level folders: the archive is not rooted at a single one.
	require.NoError(t, tarWriter.WriteHeader(&tar.Header{Name: "my-app-1.0.0-unoq/", Typeflag: tar.TypeDir, Mode: 0o755}))
	require.NoError(t, tarWriter.WriteHeader(&tar.Header{Name: "other-folder/release.yaml", Typeflag: tar.TypeReg, Mode: 0o644}))

	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzipWriter.Close())
	require.NoError(t, file.Close())

	_, err = ReadReleaseInfo(archivePath)
	assert.ErrorContains(t, err, "is not rooted at a single release folder")
}
