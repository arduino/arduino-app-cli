// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package remotebricksindex

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/platform"
)

func TestResolve(t *testing.T) {
	index := &Index{Bricks: []Brick{{
		ID:        "acme:hello",
		Namespace: "acme",
		Latest:    "1.2.0",
		Releases:  []Release{{Version: "1.0.0"}, {Version: "1.2.0"}},
	}}}

	tests := []struct {
		name          string
		index         *Index
		installed     []string
		ref           Reference
		wantVersion   string
		wantInstalled bool
		wantErr       error
	}{
		{name: "unpinned resolves to the latest of the index", index: index, ref: Reference{ID: "acme:hello"}, wantVersion: "1.2.0"},
		{name: "unpinned ignores older installed versions", index: index, installed: []string{"acme/hello@1.0.0"}, ref: Reference{ID: "acme:hello"}, wantVersion: "1.2.0"},
		{name: "unpinned installed", index: index, installed: []string{"acme/hello@1.2.0"}, ref: Reference{ID: "acme:hello"}, wantVersion: "1.2.0", wantInstalled: true},
		{name: "pinned", index: index, ref: Reference{ID: "acme:hello", Version: "1.0.0"}, wantVersion: "1.0.0"},
		{name: "pinned installed but removed from the index", index: index, installed: []string{"acme/hello@0.9.0"}, ref: Reference{ID: "acme:hello", Version: "0.9.0"}, wantErr: ErrReleaseNotFound},
		{name: "pinned neither installed nor in the index", index: index, ref: Reference{ID: "acme:hello", Version: "0.9.0"}, wantErr: ErrReleaseNotFound},
		{name: "unpinned not in the index", index: index, ref: Reference{ID: "acme:other"}, wantErr: ErrBrickNotFound},
		{name: "pinned installed without index", installed: []string{"acme/hello@1.0.0"}, ref: Reference{ID: "acme:hello", Version: "1.0.0"}, wantErr: ErrIndexNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bricksDir := installedBricks(t, tc.installed...)
			res, err := Resolve(tc.index, bricksDir, []Reference{tc.ref})
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, res, 1)
			require.Equal(t, tc.wantVersion, res[0].Version)
			require.Equal(t, tc.wantInstalled, res[0].Installed)
			require.Equal(t, tc.wantVersion, res[0].Release.Version)
			require.Equal(t, bricksDir.Join("acme", "hello@"+tc.wantVersion), res[0].Path)
		})
	}

	t.Run("installed brick removed from the index", func(t *testing.T) {
		_, err := Resolve(index, installedBricks(t, "acme/gone@1.0.0"), []Reference{{ID: "acme:gone", Version: "1.0.0"}})
		require.ErrorIs(t, err, ErrBrickNotFound)
		require.ErrorContains(t, err, "brick uninstall acme:gone@1.0.0")
	})

	t.Run("built-in and local bricks are skipped", func(t *testing.T) {
		res, err := Resolve(nil, installedBricks(t), []Reference{{ID: "arduino:camera"}, {ID: "my-local-brick"}})
		require.NoError(t, err)
		require.Empty(t, res)
	})

	t.Run("only third-party bricks can be pinned", func(t *testing.T) {
		_, err := Resolve(index, installedBricks(t), []Reference{{ID: "arduino:camera", Version: "1.0.0"}})
		require.ErrorContains(t, err, "only third-party bricks")
	})
}

func TestLoadResolved(t *testing.T) {
	bricksDir := paths.New(t.TempDir())
	brickDir := bricksDir.Join("acme", "hello@1.0.0")
	require.NoError(t, brickDir.MkdirAll())
	require.NoError(t, brickDir.Join(brickConfigFileName).WriteFile([]byte("id: acme:hello\nname: Hello\nsupported_boards: [board-a]\n")))
	require.NoError(t, brickDir.Join("brick_compose.yaml").WriteFile([]byte("services: {}\n")))
	require.NoError(t, brickDir.Join("brick_compose.board-a.yaml").WriteFile([]byte("services: {}\n")))
	hello := ResolvedBrick{ID: "acme:hello", Version: "1.0.0", Path: brickDir, Installed: true}

	t.Run("installed", func(t *testing.T) {
		bricks, err := LoadResolved(platform.Platform{BoardName: "board-a"}, []ResolvedBrick{hello})
		require.NoError(t, err)
		require.Len(t, bricks, 1)
		require.Equal(t, "Hello", bricks[0].Name)
		require.Equal(t, "acme", bricks[0].Source)
		require.Equal(t, brickDir, bricks[0].FullPath)
		require.Equal(t, brickDir.Join("brick_compose.board-a.yaml"), bricks[0].ComposeFile)
	})

	t.Run("board not supported", func(t *testing.T) {
		_, err := LoadResolved(platform.Platform{BoardName: "board-b"}, []ResolvedBrick{hello})
		require.ErrorContains(t, err, "does not support the board board-b")
	})

	t.Run("not installed", func(t *testing.T) {
		missing := ResolvedBrick{ID: "acme:hello", Version: "1.2.0", Path: bricksDir.Join("acme", "hello@1.2.0")}
		_, err := LoadResolved(platform.Platform{}, []ResolvedBrick{missing})
		require.ErrorContains(t, err, "brick install acme:hello@1.2.0")
	})
}

func TestSourceLoad(t *testing.T) {
	bricksDir := paths.New(t.TempDir())
	brickDir := bricksDir.Join("acme", "hello@1.2.0")
	require.NoError(t, brickDir.MkdirAll())
	require.NoError(t, brickDir.Join(brickConfigFileName).WriteFile([]byte("id: acme:hello\nname: Hello\n")))
	require.NoError(t, brickDir.Join("brick_compose.yaml").WriteFile([]byte("services: {}\n")))

	indexDir := paths.New(t.TempDir())
	index := `{"bricks": [
		{"id": "acme:hello", "namespace": "acme", "latest": "1.2.0", "releases": [{"version": "1.2.0"}]},
		{"id": "acme:missing", "namespace": "acme", "latest": "1.0.0", "releases": [{"version": "1.0.0"}]}
	]}`
	require.NoError(t, indexDir.Join(IndexFileName).WriteFile([]byte(index)))

	t.Run("loads the bricks it can and reports the others", func(t *testing.T) {
		src := Source{IndexDir: indexDir, BricksDir: bricksDir}
		bricks, err := src.Load([]Reference{{ID: "arduino:camera"}, {ID: "acme:hello"}, {ID: "acme:missing"}})
		require.ErrorContains(t, err, "brick install acme:missing@1.0.0")
		require.Len(t, bricks, 1)
		require.Equal(t, "acme:hello", bricks[0].ID)
	})

	t.Run("no third-party bricks do not need the index", func(t *testing.T) {
		src := Source{IndexDir: paths.New(t.TempDir()), BricksDir: bricksDir}
		bricks, err := src.Load([]Reference{{ID: "arduino:camera"}, {ID: "my-local-brick"}})
		require.NoError(t, err)
		require.Empty(t, bricks)
	})

	t.Run("missing index is reported once", func(t *testing.T) {
		src := Source{IndexDir: paths.New(t.TempDir()), BricksDir: bricksDir}
		_, err := src.Load([]Reference{{ID: "acme:hello"}, {ID: "acme:missing"}})
		require.ErrorIs(t, err, ErrIndexNotFound)
		require.Equal(t, ErrIndexNotFound.Error(), err.Error())
	})
}
