// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"testing"

	"github.com/stretchr/testify/require"
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
