// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/platform"
)

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create(name)
		require.NoError(t, err)
		_, err = f.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return buf.Bytes()
}

// cameraBrick is the brick of the releases returned by archiveRelease.
var cameraBrick = &RemoteBrick{ID: "acme:camera", Namespace: "acme", Author: "Acme", Name: "Camera"}

// archiveRelease serves archive and returns a release of acme:camera that points to it.
func archiveRelease(t *testing.T, archive []byte) *RemoteRelease {
	t.Helper()
	return archiveReleaseAt(t, archive, serveFile(t, http.StatusOK, archive, false))
}

func archiveReleaseAt(t *testing.T, archive []byte, url string) *RemoteRelease {
	t.Helper()
	sum := sha256.Sum256(archive)
	return &RemoteRelease{
		Version: semver.MustParse("1.0.0"),
		URL:     url,
		Hash:    hex.EncodeToString(sum[:]),
		Size:    int64(len(archive)),
	}
}

func TestParseBrickReference(t *testing.T) {
	id, version, err := ParseBrickReference("acme:camera")
	require.NoError(t, err)
	require.Equal(t, "acme:camera", id)
	require.Nil(t, version)

	id, version, err = ParseBrickReference("acme:camera@1.2.0")
	require.NoError(t, err)
	require.Equal(t, "acme:camera", id)
	require.Equal(t, "1.2.0", version.String())

	for _, ref := range []string{"camera", "acme:Camera", "acme:camera@", "acme:camera@v1", "acme:camera@1.0.0/.."} {
		_, _, err := ParseBrickReference(ref)
		require.Error(t, err, ref)
	}
}

func TestFindRelease(t *testing.T) {
	release := func(version string, boards ...string) RemoteRelease {
		return RemoteRelease{Version: semver.MustParse(version), SupportedBoards: boards}
	}
	index := &RemoteIndex{Bricks: []RemoteBrick{
		{ID: "acme:camera", Releases: []RemoteRelease{release("1.10.0", "ventunoq"), release("1.2.0"), release("1.9.0", "unoq", "ventunoq")}},
		{ID: "acme:audio", Releases: []RemoteRelease{release("1.0.0", "ventunoq")}},
	}}
	unoq := platform.Platform{BoardName: "unoq"}

	tests := []struct {
		name     string
		id       string
		version  string
		platform platform.Platform
		want     string
		wantErr  error
	}{
		{"latest supported release", "acme:camera", "", unoq, "1.9.0", nil},
		{"latest release", "acme:camera", "", platform.Platform{BoardName: "ventunoq"}, "1.10.0", nil},
		{"any board without a board name", "acme:camera", "", platform.Platform{}, "1.10.0", nil},
		{"given version", "acme:camera", "1.2.0", unoq, "1.2.0", nil},
		{"given version, compared as semver", "acme:camera", "1.2", unoq, "1.2.0", nil},
		{"given version not supported", "acme:camera", "1.10.0", unoq, "", ErrUnsupportedBoard},
		{"no release supported", "acme:audio", "", unoq, "", ErrUnsupportedBoard},
		{"missing version", "acme:camera", "9.9.9", unoq, "", ErrReleaseNotFound},
		{"missing brick", "acme:missing", "", unoq, "", ErrBrickNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var version *semver.Version
			if tt.version != "" {
				version = semver.MustParse(tt.version)
			}
			brick, err := index.FindBrick(tt.id)
			var release *RemoteRelease
			if err == nil {
				release, err = brick.FindRelease(version, tt.platform)
			}
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.id, brick.ID)
			require.Equal(t, tt.want, release.Version.String())
		})
	}

	camera, err := index.FindBrick("acme:camera")
	require.NoError(t, err)
	releases := camera.SupportedReleases(unoq)
	versions := make([]string, 0, len(releases))
	for _, r := range releases {
		versions = append(versions, r.Version.String())
	}
	require.Equal(t, []string{"1.2.0", "1.9.0"}, versions)
}
func TestInstall(t *testing.T) {
	bricksDir := paths.New(t.TempDir())
	const image = "ghcr.io/acme/camera:1.0@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	release := archiveRelease(t, makeZip(t, map[string]string{
		"camera/src/brick_config.yaml":  "id: acme:camera\n",
		"camera/src/brick_compose.yaml": "services:\n  camera:\n    image: " + image + "\n",
		"camera/src/README.md":          "readme",
		"camera/docs/API.md":            "api",
		"camera/examples/main.py":       "example",
		"__MACOSX/camera/._file.md":     "metadata",
	}))

	var downloaded, total int64
	progress := func(d, tot int64) { downloaded, total = d, tot }
	var prepared []string
	prepare := func(b Brick) error {
		require.NoDirExists(t, bricksDir.Join("acme", "camera", "1.0.0").String(), "prepared before it is installed")
		composeFile, ok := b.GetComposeFile()
		require.True(t, ok)
		require.FileExists(t, composeFile.String())
		prepared = append(prepared, b.ID+"@"+b.Version)
		return nil
	}
	res, err := Install(t.Context(), DownloadConfig("test"), bricksDir, cameraBrick, release, platform.Platform{}, progress, nil, prepare)
	require.NoError(t, err)
	require.False(t, res.AlreadyInstalled)
	require.Equal(t, []string{"acme:camera@1.0.0"}, prepared)
	require.Equal(t, release.Size, downloaded)
	require.Equal(t, release.Size, total)
	require.Equal(t, bricksDir.Join("acme", "camera", "1.0.0").String(), res.Path.String())
	require.FileExists(t, res.Path.Join("src", "brick_config.yaml").String())
	require.FileExists(t, res.Path.Join("src", "README.md").String())
	require.FileExists(t, res.Path.Join("docs", "API.md").String())
	require.FileExists(t, res.Path.Join("examples", "main.py").String())

	res, err = Install(t.Context(), DownloadConfig("test"), bricksDir, cameraBrick, release, platform.Platform{}, nil, nil, nil)
	require.NoError(t, err)
	require.True(t, res.AlreadyInstalled)

	// No temporary folders are left behind.
	entries, err := bricksDir.ReadDir(paths.FilterDirectories())
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestInstallFailures(t *testing.T) {
	validArchive := makeZip(t, map[string]string{"camera/src/brick_config.yaml": "id: acme:camera\n"})
	tests := []struct {
		name    string
		release func(t *testing.T) *RemoteRelease
		prepare func(Brick) error
		wantErr string
	}{
		{
			name: "hash mismatch",
			release: func(t *testing.T) *RemoteRelease {
				r := archiveRelease(t, validArchive)
				r.Hash = hex.EncodeToString(make([]byte, sha256.Size))
				return r
			},
			wantErr: "SHA-256 mismatch",
		},
		{
			name: "size mismatch",
			release: func(t *testing.T) *RemoteRelease {
				r := archiveRelease(t, validArchive)
				r.Size--
				return r
			},
			wantErr: "the server declares",
		},
		{
			name: "smaller archive without declared size",
			release: func(t *testing.T) *RemoteRelease {
				r := archiveReleaseAt(t, validArchive, serveFile(t, http.StatusOK, validArchive, true))
				r.Size++
				return r
			},
			wantErr: "size mismatch",
		},
		{
			name: "larger archive without declared size",
			release: func(t *testing.T) *RemoteRelease {
				r := archiveReleaseAt(t, validArchive, serveFile(t, http.StatusOK, validArchive, true))
				r.Size--
				return r
			},
			wantErr: "larger than",
		},
		{
			name: "server error",
			release: func(t *testing.T) *RemoteRelease {
				return archiveReleaseAt(t, validArchive, serveFile(t, http.StatusNotFound, nil, false))
			},
			wantErr: "404",
		},
		{
			name: "not enough disk space",
			release: func(t *testing.T) *RemoteRelease {
				r := archiveRelease(t, validArchive)
				r.Size = 1 << 62
				return r
			},
			wantErr: "not enough disk space",
		},
		{
			name: "invalid hash",
			release: func(t *testing.T) *RemoteRelease {
				r := archiveRelease(t, validArchive)
				r.Hash = "SHA-256:" + r.Hash
				return r
			},
			wantErr: "invalid SHA-256 hash",
		},
		{
			name: "unsupported board",
			release: func(t *testing.T) *RemoteRelease {
				r := archiveRelease(t, validArchive)
				r.SupportedBoards = []string{"ventunoq"}
				return r
			},
			wantErr: "not supported by this board",
		},
		{
			name: "multiple root folders",
			release: func(t *testing.T) *RemoteRelease {
				return archiveRelease(t, makeZip(t, map[string]string{
					"camera/src/brick_config.yaml": "id: acme:camera\n",
					"other/file":                   "x",
				}))
			},
			wantErr: "single root folder",
		},
		{
			name: "file at the root",
			release: func(t *testing.T) *RemoteRelease {
				return archiveRelease(t, makeZip(t, map[string]string{"brick_config.yaml": "id: acme:camera\n"}))
			},
			wantErr: "single root folder",
		},
		{
			name: "path traversal",
			release: func(t *testing.T) *RemoteRelease {
				return archiveRelease(t, makeZip(t, map[string]string{
					"camera/src/brick_config.yaml": "id: acme:camera\n",
					"camera/../../evil":            "x",
				}))
			},
			wantErr: "invalid entry",
		},
		{
			name: "missing brick_config.yaml",
			release: func(t *testing.T) *RemoteRelease {
				return archiveRelease(t, makeZip(t, map[string]string{"camera/src/README.md": "readme"}))
			},
			wantErr: "does not contain a valid src/brick_config.yaml",
		},
		{
			name: "brick_config.yaml outside src",
			release: func(t *testing.T) *RemoteRelease {
				return archiveRelease(t, makeZip(t, map[string]string{"camera/brick_config.yaml": "id: acme:camera\n"}))
			},
			wantErr: "does not contain a valid src/brick_config.yaml",
		},
		{
			name: "wrong brick id",
			release: func(t *testing.T) *RemoteRelease {
				return archiveRelease(t, makeZip(t, map[string]string{"camera/src/brick_config.yaml": "id: other:camera\n"}))
			},
			wantErr: `declares id "other:camera"`,
		},
		{
			name:    "prepare fails",
			release: func(t *testing.T) *RemoteRelease { return archiveRelease(t, validArchive) },
			prepare: func(Brick) error { return errors.New("cannot pull") },
			wantErr: "cannot pull",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bricksDir := paths.New(t.TempDir())
			_, err := Install(t.Context(), DownloadConfig("test"), bricksDir, cameraBrick, tc.release(t), platform.Platform{BoardName: "unoq"}, nil, nil, tc.prepare)
			require.ErrorContains(t, err, tc.wantErr)

			// Nothing is installed and no temporary folders are left behind.
			entries, err := bricksDir.ReadDir(paths.FilterDirectories())
			require.NoError(t, err)
			require.Empty(t, entries)
		})
	}
}
