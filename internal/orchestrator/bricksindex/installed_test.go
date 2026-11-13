// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/platform"
)

func writeInstalledBrick(t *testing.T, bricksDir *paths.Path, folder, config string) {
	t.Helper()
	dir := bricksDir.Join(folder)
	require.NoError(t, dir.MkdirAll())
	if config != "" {
		require.NoError(t, dir.Join(releaseSrcDir).MkdirAll())
		require.NoError(t, dir.Join(releaseSrcDir, brickConfigFileName).WriteFile([]byte(config)))
	}
}

func TestInstalledBricks(t *testing.T) {
	bricksDir := paths.New(t.TempDir())
	write := func(folder, config string) { writeInstalledBrick(t, bricksDir, folder, config) }
	write("acme/camera/1.10.0", "id: acme:camera\nname: Camera\n")
	write("acme/camera/1.2.0", "id: acme:camera\nname: Camera\n")
	write("acme/camera/1.11.0", "id: acme:other\n") // declares another id: the previous version is used
	write("acme/audio/1.0.0", "id: acme:audio\nname: Audio\n")
	write("acme/wrong/1.0.0", "id: acme:other\n")         // declares another id
	write("acme/missing/1.0.0", "")                       // no brick_config.yaml
	write("acme/camera/notes", "id: acme:camera\n")       // not created by install
	write("acme/camera@1.0.0", "id: acme:camera\n")       // old layout
	write("acme/Camera/1.0.0", "id: acme:Camera\n")       // invalid name
	write(".install-123/x/1.0.0", "id: .install-123:x\n") // temporary folder

	installed := (&BricksIndex{}).WithInstalledBricks(bricksDir).installedBricks()
	got := make([]string, 0, len(installed))
	for _, b := range installed {
		require.Equal(t, "acme", b.Source)
		got = append(got, b.ID+"@"+b.Version+" "+b.Name)
	}
	require.Equal(t, []string{
		"acme:audio@1.0.0 Audio",
		"acme:camera@1.10.0 Camera",
	}, got)
	audioDir := bricksDir.Join("acme", "audio", "1.0.0")
	require.Equal(t, audioDir.Join("src").String(), installed[0].FullPath.String())
	require.Equal(t, audioDir.Join("src", "README.md").String(), installed[0].ReadmeFile.String())
	require.Equal(t, audioDir.Join("examples").String(), installed[0].ExamplesPath.String())
	require.Equal(t, audioDir.Join("docs", "API.md").String(), installed[0].DocsAPIPath.String())

	require.Empty(t, (&BricksIndex{}).WithInstalledBricks(bricksDir.Join("missing")).installedBricks())
}

func TestBricksIndexWithInstalledBricks(t *testing.T) {
	bricksDir := paths.New(t.TempDir())
	write := func(folder, config string) { writeInstalledBrick(t, bricksDir, folder, config) }
	write("acme/camera/1.2.0", "id: acme:camera\nname: Camera\n")
	write("acme/camera/1.10.0", "id: acme:camera\nname: Camera\nsupported_boards: [unoq]\n")
	write("acme/camera/2.0.0", "id: acme:camera\nname: Camera\nsupported_boards: [ventunoq]\n")
	write("acme/other_board/1.0.0", "id: acme:other_board\nname: Other\nsupported_boards: [ventunoq]\n")
	write("arduino/builtin/1.0.0", "id: arduino:builtin\nname: Installed copy\n")

	index := (&BricksIndex{
		BuiltInBricks: []Brick{{ID: "arduino:builtin", Name: "Built-in"}},
		platform:      platform.Platform{BoardName: "unoq"},
	}).WithInstalledBricks(bricksDir)

	t.Run("latest supported version", func(t *testing.T) {
		brick, ok := index.FindBrickByID("acme:camera")
		require.True(t, ok)
		require.Equal(t, "1.10.0", brick.Version)
		require.Equal(t, "acme", brick.Source)
	})

	t.Run("no supported version", func(t *testing.T) {
		_, ok := index.FindBrickByID("acme:other_board")
		require.False(t, ok)
	})

	t.Run("built-in bricks take precedence", func(t *testing.T) {
		brick, ok := index.FindBrickByID("arduino:builtin")
		require.True(t, ok)
		require.Equal(t, "Built-in", brick.Name)
	})

	t.Run("list", func(t *testing.T) {
		bricks := index.ListBricks()
		got := make([]string, 0, len(bricks))
		for _, b := range bricks {
			got = append(got, b.ID+"@"+b.Version)
		}
		require.ElementsMatch(t, []string{"arduino:builtin@", "acme:camera@1.10.0"}, got)
	})

	t.Run("all installed versions", func(t *testing.T) {
		bricks := index.AllInstalledBricks()
		got := make([]string, 0, len(bricks))
		for _, b := range bricks {
			got = append(got, b.ID+"@"+b.Version)
		}
		require.Equal(t, []string{
			"acme:camera@1.2.0", "acme:camera@1.10.0", "acme:camera@2.0.0",
			"acme:other_board@1.0.0",
			"arduino:builtin@1.0.0",
		}, got)
	})

	t.Run("bricks installed after loading are found", func(t *testing.T) {
		write("acme/audio/1.0.0", "id: acme:audio\nname: Audio\n")
		brick, ok := index.FindBrickByID("acme:audio")
		require.True(t, ok)
		require.Equal(t, "1.0.0", brick.Version)
	})

	t.Run("not included without WithInstalledBricks", func(t *testing.T) {
		index := &BricksIndex{}
		_, ok := index.FindBrickByID("acme:camera")
		require.False(t, ok)
		require.Empty(t, index.ListBricks())
	})
}
