// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/platform"
)

// indexRelease is a release of the index that is never downloaded.
func indexRelease(version string, boards ...string) RemoteRelease {
	return RemoteRelease{
		Version:         semver.MustParse(version),
		URL:             "https://example.com/brick.zip",
		Hash:            strings.Repeat("0", 64),
		Size:            1,
		SupportedBoards: boards,
	}
}

// saveIndex saves the index in a new folder, which it returns.
func saveIndex(t *testing.T, bricks ...RemoteBrick) *paths.Path {
	t.Helper()
	dir := paths.New(t.TempDir())
	data, err := json.Marshal(RemoteIndex{Bricks: bricks})
	require.NoError(t, err)
	require.NoError(t, dir.Join(RemoteIndexFileName).WriteFile(data))
	return dir
}

// newIndex returns an index with the bricks installed in bricksDir, checked against the index saved in indexDir.
func newIndex(bricksDir, indexDir *paths.Path, board string) *BricksIndex {
	return (&BricksIndex{platform: platform.Platform{BoardName: board}}).WithInstalledBricks(bricksDir).WithRemoteIndex(indexDir)
}

// installBrick installs a version of the brick in bricksDir, with the given config and a compose file.
func installBrick(t *testing.T, bricksDir *paths.Path, id, version, config string) {
	t.Helper()
	namespace, name, err := ParseBrickID(id)
	require.NoError(t, err)
	srcDir := bricksDir.Join(namespace, name, version, releaseSrcDir, name)
	require.NoError(t, srcDir.MkdirAll())
	require.NoError(t, srcDir.Join(brickConfigFileName).WriteFile([]byte(config)))
	require.NoError(t, srcDir.Join("brick_compose.yaml").WriteFile([]byte("services: {}\n")))
}

func TestResolve(t *testing.T) {
	index := &RemoteIndex{Bricks: []RemoteBrick{{
		ID:        "acme:hello",
		Namespace: "acme",
		Releases:  []RemoteRelease{indexRelease("1.0.0"), indexRelease("1.2.0"), indexRelease("1.3.0", "ventunoq")},
	}}}

	tests := []struct {
		name string
		// installed are the installed versions of acme:hello, a version followed by
		// ":board" supports only that board.
		installed     []string
		ref           Reference
		wantVersion   string
		wantInstalled bool
		wantErr       error
	}{
		{name: "unpinned resolves to the latest of the index that supports the board", ref: Reference{ID: "acme:hello"}, wantVersion: "1.2.0"},
		{name: "unpinned sticks to the installed version", installed: []string{"1.0.0"}, ref: Reference{ID: "acme:hello"}, wantVersion: "1.0.0", wantInstalled: true},
		{name: "unpinned uses the latest installed version", installed: []string{"1.0.0", "1.2.0"}, ref: Reference{ID: "acme:hello"}, wantVersion: "1.2.0", wantInstalled: true},
		{name: "unpinned skips the installed versions that do not support the board", installed: []string{"1.0.0", "1.3.0:ventunoq"}, ref: Reference{ID: "acme:hello"}, wantVersion: "1.0.0", wantInstalled: true},
		{name: "unpinned installed but removed from the index", installed: []string{"0.9.0"}, ref: Reference{ID: "acme:hello"}, wantErr: ErrReleaseNotFound},
		{name: "unpinned not in the index", ref: Reference{ID: "acme:other"}, wantErr: ErrBrickNotFound},
		{name: "pinned", ref: Reference{ID: "acme:hello", Version: "1.0.0"}, wantVersion: "1.0.0"},
		{name: "pinned installed", installed: []string{"1.0.0", "1.2.0"}, ref: Reference{ID: "acme:hello", Version: "1.0.0"}, wantVersion: "1.0.0", wantInstalled: true},
		{name: "pinned to a version that does not support the board", ref: Reference{ID: "acme:hello", Version: "1.3.0"}, wantErr: ErrUnsupportedBoard},
		{name: "pinned installed to a version that does not support the board", installed: []string{"1.3.0:ventunoq"}, ref: Reference{ID: "acme:hello", Version: "1.3.0"}, wantErr: ErrUnsupportedBoard},
		{name: "pinned installed but removed from the index", installed: []string{"0.9.0"}, ref: Reference{ID: "acme:hello", Version: "0.9.0"}, wantErr: ErrReleaseNotFound},
		{name: "pinned neither installed nor in the index", ref: Reference{ID: "acme:hello", Version: "0.9.0"}, wantErr: ErrReleaseNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bricksDir := paths.New(t.TempDir())
			for _, installed := range tc.installed {
				version, board, _ := strings.Cut(installed, ":")
				config := "id: acme:hello\nname: Hello\n"
				if board != "" {
					config += "supported_boards: [" + board + "]\n"
				}
				installBrick(t, bricksDir, "acme:hello", version, config)
			}
			res, err := newIndex(bricksDir, nil, "unoq").resolve(index, tc.ref)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantVersion, res.version.String())
			require.Equal(t, tc.wantVersion, res.release.Version.String())
			if !tc.wantInstalled {
				require.Nil(t, res.installed)
				return
			}
			require.NotNil(t, res.installed)
			require.Equal(t, tc.wantVersion, res.installed.Version)
			require.Equal(t, bricksDir.Join("acme", "hello", tc.wantVersion, releaseSrcDir, "hello"), res.installed.FullPath)
		})
	}

	t.Run("installed brick removed from the index", func(t *testing.T) {
		bricksDir := paths.New(t.TempDir())
		installBrick(t, bricksDir, "acme:gone", "1.0.0", "id: acme:gone\n")
		_, err := newIndex(bricksDir, nil, "unoq").resolve(index, Reference{ID: "acme:gone", Version: "1.0.0"})
		require.ErrorIs(t, err, ErrBrickNotFound)
		require.ErrorContains(t, err, "brick uninstall acme:gone@1.0.0")
	})
}

func TestWithApp(t *testing.T) {
	bricksDir := paths.New(t.TempDir())
	installBrick(t, bricksDir, "acme:hello", "1.2.0", "id: acme:hello\nname: Hello\n")
	installBrick(t, bricksDir, "acme:delisted", "0.9.0", "id: acme:delisted\n")
	indexDir := saveIndex(t,
		RemoteBrick{ID: "acme:hello", Namespace: "acme", Releases: []RemoteRelease{indexRelease("1.2.0")}},
		RemoteBrick{ID: "acme:missing", Namespace: "acme", Releases: []RemoteRelease{indexRelease("1.0.0")}},
		RemoteBrick{ID: "acme:delisted", Namespace: "acme", Releases: []RemoteRelease{indexRelease("1.0.0")}},
	)
	board := newIndex(bricksDir, indexDir, "")
	board.BuiltInBricks = []Brick{{ID: "arduino:camera"}}

	t.Run("loads the bricks it can and reports the others", func(t *testing.T) {
		appIndex, err := board.WithApp([]Brick{{ID: "my-local-brick"}}, []Reference{{ID: "arduino:camera"}, {ID: "acme:hello"}, {ID: "acme:missing"}, {ID: "acme:delisted"}})
		require.ErrorIs(t, err, ErrNotInstalled)
		require.ErrorContains(t, err, "brick install acme:missing@1.0.0")
		require.ErrorIs(t, err, ErrReleaseNotFound)

		hello, found := appIndex.FindBrickByID("acme:hello")
		require.True(t, found)
		require.Equal(t, "Hello", hello.Name)
		require.Equal(t, "1.2.0", hello.Version)
		require.Equal(t, "acme", hello.Source)
		for _, id := range []string{"my-local-brick", "arduino:camera"} {
			_, found = appIndex.FindBrickByID(id)
			require.True(t, found, id)
		}
		// The installed version the index no longer lists is not used in its place.
		_, found = appIndex.FindBrickByID("acme:delisted")
		require.False(t, found)

		_, found = board.FindBrickByID("my-local-brick")
		require.False(t, found)
	})

	t.Run("no third-party bricks do not need the index", func(t *testing.T) {
		_, err := newIndex(bricksDir, nil, "").WithApp(nil, []Reference{{ID: "arduino:camera"}, {ID: "my-local-brick"}})
		require.NoError(t, err)
	})

	t.Run("installed bricks need the index", func(t *testing.T) {
		appIndex, err := newIndex(bricksDir, paths.New(t.TempDir()), "").WithApp(nil, []Reference{{ID: "acme:hello"}})
		require.ErrorIs(t, err, ErrIndexNotFound)
		_, found := appIndex.FindBrickByID("acme:hello")
		require.False(t, found)
	})

	t.Run("missing index is reported once", func(t *testing.T) {
		_, err := newIndex(bricksDir, paths.New(t.TempDir()), "").WithApp(nil, []Reference{{ID: "acme:missing"}, {ID: "acme:other"}})
		require.ErrorIs(t, err, ErrIndexNotFound)
		require.Equal(t, ErrIndexNotFound.Error(), err.Error())
	})

	t.Run("only third-party bricks can be pinned", func(t *testing.T) {
		_, err := board.WithApp(nil, []Reference{{ID: "arduino:camera", Version: "1.0.0"}})
		require.ErrorContains(t, err, "only third-party bricks")
	})
}

func TestLookupReference(t *testing.T) {
	bricksDir := paths.New(t.TempDir())
	installBrick(t, bricksDir, "acme:hello", "1.2.0", "id: acme:hello\nname: Hello\n")
	indexDir := saveIndex(t,
		RemoteBrick{ID: "acme:hello", Namespace: "acme", Releases: []RemoteRelease{indexRelease("1.2.0")}},
		RemoteBrick{ID: "acme:missing", Namespace: "acme", Releases: []RemoteRelease{indexRelease("1.0.0")}},
	)
	index := newIndex(bricksDir, indexDir, "")

	t.Run("installed brick", func(t *testing.T) {
		brick, err := index.LookupReference(Reference{ID: "acme:hello", Version: "1.2.0"})
		require.NoError(t, err)
		require.Equal(t, "Hello", brick.Name)
	})

	t.Run("brick listed in the index but not installed", func(t *testing.T) {
		brick, err := index.LookupReference(Reference{ID: "acme:missing"})
		require.NoError(t, err)
		require.Nil(t, brick)
	})

	t.Run("version not listed in the index", func(t *testing.T) {
		_, err := index.LookupReference(Reference{ID: "acme:missing", Version: "2.0.0"})
		require.ErrorIs(t, err, ErrReleaseNotFound)
	})

	t.Run("brick not listed in the index", func(t *testing.T) {
		_, err := index.LookupReference(Reference{ID: "acme:other"})
		require.ErrorIs(t, err, ErrBrickNotFound)
	})

	t.Run("installed brick without the index", func(t *testing.T) {
		_, err := newIndex(bricksDir, paths.New(t.TempDir()), "").LookupReference(Reference{ID: "acme:hello"})
		require.ErrorIs(t, err, ErrIndexNotFound)
	})
}

func TestInstallReferences(t *testing.T) {
	release := archiveRelease(t, makeZip(t, map[string]string{
		"src/hello/brick_config.yaml":  "id: acme:hello\n",
		"src/hello/brick_compose.yaml": "services: {}\n",
	}))
	data, err := json.Marshal(RemoteIndex{Bricks: []RemoteBrick{{ID: "acme:hello", Namespace: "acme", Releases: []RemoteRelease{*release}}}})
	require.NoError(t, err)
	serveIndex := func(t *testing.T) *url.URL { return mustParseURL(t, serveFile(t, http.StatusOK, data, false)) }
	refs := []Reference{{ID: "arduino:camera"}, {ID: "acme:hello"}}
	config := DownloadConfig("test")
	install := func(bricksDir *paths.Path) func(*RemoteBrick, *RemoteRelease) error {
		return func(b *RemoteBrick, r *RemoteRelease) error {
			_, err := Install(t.Context(), config, bricksDir, b, r, platform.Platform{}, nil, nil)
			return err
		}
	}
	installed := func(t *testing.T, version string) *paths.Path {
		bricksDir := paths.New(t.TempDir())
		installBrick(t, bricksDir, "acme:hello", version, "id: acme:hello\n")
		return bricksDir
	}

	t.Run("downloads the missing index and installs the missing bricks", func(t *testing.T) {
		index := newIndex(paths.New(t.TempDir()), paths.New(t.TempDir()), "")
		var messages []string
		err := index.InstallReferences(t.Context(), config, serveIndex(t), refs, install(index.installedDir), func(msg string) { messages = append(messages, msg) })
		require.NoError(t, err)
		require.Equal(t, []string{"updating the bricks index", "installing the brick acme:hello@1.0.0"}, messages)

		appIndex, err := index.WithApp(nil, refs)
		require.NoError(t, err)
		_, found := appIndex.FindBrickByID("acme:hello")
		require.True(t, found)
	})

	t.Run("does not access the network when the index and the bricks are on the board", func(t *testing.T) {
		indexDir := paths.New(t.TempDir())
		require.NoError(t, indexDir.Join(RemoteIndexFileName).WriteFile(data))
		index := newIndex(installed(t, "1.0.0"), indexDir, "")
		require.NoError(t, index.InstallReferences(t.Context(), config, mustParseURL(t, "http://127.0.0.1:0/brick-index.json"), refs, install(index.installedDir), func(string) {}))
	})

	t.Run("downloads a missing index even when the bricks are installed", func(t *testing.T) {
		index := newIndex(installed(t, "1.0.0"), paths.New(t.TempDir()), "")
		var messages []string
		require.NoError(t, index.InstallReferences(t.Context(), config, serveIndex(t), refs, install(index.installedDir), func(msg string) { messages = append(messages, msg) }))
		require.Equal(t, []string{"updating the bricks index"}, messages)
		require.FileExists(t, index.remoteIndexDir.Join(RemoteIndexFileName).String())
	})

	t.Run("removing the index does not skip the check of the versions it no longer lists", func(t *testing.T) {
		index := newIndex(installed(t, "0.5.0"), paths.New(t.TempDir()), "")
		err := index.InstallReferences(t.Context(), config, serveIndex(t), refs, install(index.installedDir), func(string) {})
		require.ErrorIs(t, err, ErrReleaseNotFound)
		require.ErrorContains(t, err, "brick uninstall acme:hello@0.5.0")
	})

	t.Run("does not update a recent index that does not list the brick", func(t *testing.T) {
		index := newIndex(paths.New(t.TempDir()), saveIndex(t), "")
		err := index.InstallReferences(t.Context(), config, mustParseURL(t, "http://127.0.0.1:0/brick-index.json"), refs, install(index.installedDir), func(string) {})
		require.ErrorIs(t, err, ErrBrickNotFound)
		require.ErrorContains(t, err, "brick update-index")
	})

	stale := func(t *testing.T, indexDir *paths.Path) *paths.Path {
		old := time.Now().Add(-RemoteIndexMaxAge - time.Minute)
		require.NoError(t, os.Chtimes(indexDir.Join(RemoteIndexFileName).String(), old, old))
		return indexDir
	}

	t.Run("updates an old index", func(t *testing.T) {
		index := newIndex(paths.New(t.TempDir()), stale(t, saveIndex(t)), "")
		var messages []string
		require.NoError(t, index.InstallReferences(t.Context(), config, serveIndex(t), refs, install(index.installedDir), func(msg string) { messages = append(messages, msg) }))
		require.Equal(t, []string{"updating the bricks index", "installing the brick acme:hello@1.0.0"}, messages)
	})

	t.Run("uses an old index when it cannot be updated", func(t *testing.T) {
		indexDir := paths.New(t.TempDir())
		require.NoError(t, indexDir.Join(RemoteIndexFileName).WriteFile(data))
		index := newIndex(installed(t, "1.0.0"), stale(t, indexDir), "")
		var messages []string
		require.NoError(t, index.InstallReferences(t.Context(), config, mustParseURL(t, serveFile(t, http.StatusNotFound, nil, false)), refs, install(index.installedDir), func(msg string) { messages = append(messages, msg) }))
		require.Len(t, messages, 2)
		require.Contains(t, messages[1], "cannot update the bricks index, using the saved one")
	})

	t.Run("fails when the index cannot be updated", func(t *testing.T) {
		index := newIndex(installed(t, "1.0.0"), paths.New(t.TempDir()), "")
		err := index.InstallReferences(t.Context(), config, mustParseURL(t, serveFile(t, http.StatusNotFound, nil, false)), refs, install(index.installedDir), func(string) {})
		require.ErrorContains(t, err, "updating bricks index")
	})
}
