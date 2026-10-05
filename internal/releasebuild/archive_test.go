// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package releasebuild

import (
	"os"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/pkg/release"
)

func TestResolveArchivePath(t *testing.T) {
	fileName := "my-app-1.0.0-unoq" + release.ReleaseArchiveExt
	existing := paths.New(t.TempDir()).Join("taken" + release.ReleaseArchiveExt)
	require.NoError(t, existing.WriteFile(nil))
	outputDir := paths.New(t.TempDir())

	t.Run("the default is the file name in the current dir", func(t *testing.T) {
		archivePath, err := ResolveArchivePath(fileName, nil, false)
		require.NoError(t, err)
		cwd, err := os.Getwd()
		require.NoError(t, err)
		assert.Equal(t, paths.New(cwd, fileName).String(), archivePath.String())
	})

	t.Run("an output dir holds the file name", func(t *testing.T) {
		archivePath, err := ResolveArchivePath(fileName, outputDir, false)
		require.NoError(t, err)
		assert.Equal(t, outputDir.Join(fileName).String(), archivePath.String())
	})

	t.Run("an output file is the archive", func(t *testing.T) {
		wanted := outputDir.Join("named" + release.ReleaseArchiveExt)
		archivePath, err := ResolveArchivePath(fileName, wanted, false)
		require.NoError(t, err)
		assert.Equal(t, wanted.String(), archivePath.String())
	})

	t.Run("an existing archive is not overwritten", func(t *testing.T) {
		_, err := ResolveArchivePath(fileName, existing, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("overwrite takes an existing archive", func(t *testing.T) {
		archivePath, err := ResolveArchivePath(fileName, existing, true)
		require.NoError(t, err)
		assert.Equal(t, existing.String(), archivePath.String())
	})
}
