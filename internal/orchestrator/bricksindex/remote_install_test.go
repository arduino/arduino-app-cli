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
	"net/http"
	"net/http/httptest"
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

// archiveRelease serves archive and returns a release of acme:camera that points to it.
func archiveRelease(t *testing.T, archive []byte) *RemoteBrick {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	}))
	t.Cleanup(srv.Close)
	sum := sha256.Sum256(archive)
	return &RemoteBrick{
		ID:        "acme:camera",
		Namespace: "acme",
		Name:      "Camera",
		Version:   semver.MustParse("1.0.0"),
		URL:       srv.URL + "/brick.zip",
		Hash:      hex.EncodeToString(sum[:]),
		Size:      int64(len(archive)),
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
	release := func(version string, boards ...string) RemoteBrick {
		return RemoteBrick{ID: "acme:camera", Version: semver.MustParse(version), SupportedBoards: boards}
	}
	index := &RemoteIndex{Bricks: []RemoteBrick{
		release("1.10.0", "ventunoq"),
		release("1.2.0"),
		release("1.9.0", "unoq", "ventunoq"),
		{ID: "acme:audio", Version: semver.MustParse("1.0.0"), SupportedBoards: []string{"ventunoq"}},
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
			got, err := index.FindRelease(tt.id, version, tt.platform)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.id, got.ID)
			require.Equal(t, tt.want, got.Version.String())
		})
	}

	releases := index.Releases("acme:camera", unoq)
	versions := make([]string, 0, len(releases))
	for _, r := range releases {
		versions = append(versions, r.Version.String())
	}
	require.Equal(t, []string{"1.2.0", "1.9.0"}, versions)
}
func TestInstall(t *testing.T) {
	bricksDir := paths.New(t.TempDir())
	brick := archiveRelease(t, makeZip(t, map[string]string{
		"camera/brick_config.yaml":  "id: acme:camera\n",
		"camera/README.md":          "readme",
		"__MACOSX/camera/._file.md": "metadata",
	}))

	res, err := Install(t.Context(), http.DefaultClient, bricksDir, brick, platform.Platform{}, "test")
	require.NoError(t, err)
	require.False(t, res.AlreadyInstalled)
	require.Equal(t, bricksDir.Join("acme", "camera", "1.0.0").String(), res.Path.String())
	require.FileExists(t, res.Path.Join("brick_config.yaml").String())
	require.FileExists(t, res.Path.Join("README.md").String())

	res, err = Install(t.Context(), http.DefaultClient, bricksDir, brick, platform.Platform{}, "test")
	require.NoError(t, err)
	require.True(t, res.AlreadyInstalled)

	// No temporary folders are left behind.
	entries, err := bricksDir.ReadDir(paths.FilterDirectories())
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestInstallFailures(t *testing.T) {
	validArchive := makeZip(t, map[string]string{"camera/brick_config.yaml": "id: acme:camera\n"})
	tests := []struct {
		name    string
		brick   func(t *testing.T) *RemoteBrick
		wantErr string
	}{
		{
			name: "hash mismatch",
			brick: func(t *testing.T) *RemoteBrick {
				r := archiveRelease(t, validArchive)
				r.Hash = hex.EncodeToString(make([]byte, sha256.Size))
				return r
			},
			wantErr: "SHA-256 mismatch",
		},
		{
			name: "size mismatch",
			brick: func(t *testing.T) *RemoteBrick {
				r := archiveRelease(t, validArchive)
				r.Size--
				return r
			},
			wantErr: "size mismatch",
		},
		{
			name: "not enough disk space",
			brick: func(t *testing.T) *RemoteBrick {
				r := archiveRelease(t, validArchive)
				r.Size = 1 << 62
				return r
			},
			wantErr: "not enough disk space",
		},
		{
			name: "invalid hash",
			brick: func(t *testing.T) *RemoteBrick {
				r := archiveRelease(t, validArchive)
				r.Hash = "SHA-256:" + r.Hash
				return r
			},
			wantErr: "invalid SHA-256 hash",
		},
		{
			name: "unsupported board",
			brick: func(t *testing.T) *RemoteBrick {
				r := archiveRelease(t, validArchive)
				r.SupportedBoards = []string{"ventunoq"}
				return r
			},
			wantErr: "not supported by this board",
		},
		{
			name: "multiple root folders",
			brick: func(t *testing.T) *RemoteBrick {
				return archiveRelease(t, makeZip(t, map[string]string{
					"camera/brick_config.yaml": "id: acme:camera\n",
					"other/file":               "x",
				}))
			},
			wantErr: "single root folder",
		},
		{
			name: "file at the root",
			brick: func(t *testing.T) *RemoteBrick {
				return archiveRelease(t, makeZip(t, map[string]string{"brick_config.yaml": "id: acme:camera\n"}))
			},
			wantErr: "single root folder",
		},
		{
			name: "path traversal",
			brick: func(t *testing.T) *RemoteBrick {
				return archiveRelease(t, makeZip(t, map[string]string{
					"camera/brick_config.yaml": "id: acme:camera\n",
					"camera/../../evil":        "x",
				}))
			},
			wantErr: "invalid entry",
		},
		{
			name: "missing brick_config.yaml",
			brick: func(t *testing.T) *RemoteBrick {
				return archiveRelease(t, makeZip(t, map[string]string{"camera/README.md": "readme"}))
			},
			wantErr: "does not contain a valid brick_config.yaml",
		},
		{
			name: "wrong brick id",
			brick: func(t *testing.T) *RemoteBrick {
				return archiveRelease(t, makeZip(t, map[string]string{"camera/brick_config.yaml": "id: other:camera\n"}))
			},
			wantErr: `declares id "other:camera"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bricksDir := paths.New(t.TempDir())
			_, err := Install(t.Context(), http.DefaultClient, bricksDir, tc.brick(t), platform.Platform{BoardName: "unoq"}, "test")
			require.ErrorContains(t, err, tc.wantErr)

			// Nothing is installed and no temporary folders are left behind.
			entries, err := bricksDir.ReadDir(paths.FilterDirectories())
			require.NoError(t, err)
			require.Empty(t, entries)
		})
	}
}
