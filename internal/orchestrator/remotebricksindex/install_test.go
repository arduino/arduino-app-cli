// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package remotebricksindex

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

// archiveRelease serves archive and returns a release that points to it.
func archiveRelease(t *testing.T, archive []byte) *Release {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	}))
	t.Cleanup(srv.Close)
	sum := sha256.Sum256(archive)
	return &Release{
		Version: "1.0.0",
		URL:     srv.URL + "/brick.zip",
		Hash:    hex.EncodeToString(sum[:]),
		Size:    int64(len(archive)),
	}
}

func TestParseBrickReference(t *testing.T) {
	id, version, err := ParseBrickReference("acme:camera@1.1.0")
	require.NoError(t, err)
	require.Equal(t, "acme:camera", id)
	require.Equal(t, "1.1.0", version)

	id, version, err = ParseBrickReference("acme:camera")
	require.NoError(t, err)
	require.Equal(t, "acme:camera", id)
	require.Empty(t, version)

	for _, ref := range []string{"camera", "acme:camera@"} {
		_, _, err := ParseBrickReference(ref)
		require.Error(t, err, ref)
	}
}

func TestFindRelease(t *testing.T) {
	index, err := parse([]byte(validIndex))
	require.NoError(t, err)

	brick, release, err := index.FindRelease("acme:camera", "")
	require.NoError(t, err)
	require.Equal(t, "acme:camera", brick.ID)
	require.Equal(t, "1.0.0", release.Version)

	_, _, err = index.FindRelease("acme:camera", "9.9.9")
	require.ErrorIs(t, err, ErrReleaseNotFound)

	_, _, err = index.FindRelease("acme:missing", "")
	require.ErrorIs(t, err, ErrBrickNotFound)
}

func TestInstallPathRejectsUnsafeNames(t *testing.T) {
	dir := paths.New(t.TempDir())
	tests := []struct {
		id, version string
	}{
		{"acme:../camera", "1.0.0"},
		{"acme:.camera", "1.0.0"},
		{"acme:cam@era", "1.0.0"},
		{"acme:camera", "1.0.0/.."},
		{"acme:camera", ""},
	}
	for _, tc := range tests {
		_, err := InstallPath(dir, &Brick{ID: tc.id}, &Release{Version: tc.version})
		require.Error(t, err, "%s@%s", tc.id, tc.version)
	}
}

func TestInstall(t *testing.T) {
	bricksDir := paths.New(t.TempDir())
	brick := &Brick{ID: "acme:camera"}
	release := archiveRelease(t, makeZip(t, map[string]string{
		"camera/brick_config.yaml":  "id: acme:camera\n",
		"camera/README.md":          "readme",
		"__MACOSX/camera/._file.md": "metadata",
	}))

	res, err := Install(t.Context(), http.DefaultClient, bricksDir, brick, release, "test")
	require.NoError(t, err)
	require.False(t, res.AlreadyInstalled)
	require.Equal(t, bricksDir.Join("acme", "camera@1.0.0").String(), res.Path.String())
	require.FileExists(t, res.Path.Join("brick_config.yaml").String())
	require.FileExists(t, res.Path.Join("README.md").String())

	res, err = Install(t.Context(), http.DefaultClient, bricksDir, brick, release, "test")
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
		release func(t *testing.T) *Release
		wantErr string
	}{
		{
			name: "hash mismatch",
			release: func(t *testing.T) *Release {
				r := archiveRelease(t, validArchive)
				r.Hash = hex.EncodeToString(make([]byte, sha256.Size))
				return r
			},
			wantErr: "SHA-256 mismatch",
		},
		{
			name: "size mismatch",
			release: func(t *testing.T) *Release {
				r := archiveRelease(t, validArchive)
				r.Size--
				return r
			},
			wantErr: "size mismatch",
		},
		{
			name: "not enough disk space",
			release: func(t *testing.T) *Release {
				r := archiveRelease(t, validArchive)
				r.Size = 1 << 62
				return r
			},
			wantErr: "not enough disk space",
		},
		{
			name: "invalid hash",
			release: func(t *testing.T) *Release {
				r := archiveRelease(t, validArchive)
				r.Hash = "SHA-256:" + r.Hash
				return r
			},
			wantErr: "invalid SHA-256 hash",
		},
		{
			name: "multiple root folders",
			release: func(t *testing.T) *Release {
				return archiveRelease(t, makeZip(t, map[string]string{
					"camera/brick_config.yaml": "id: acme:camera\n",
					"other/file":               "x",
				}))
			},
			wantErr: "single root folder",
		},
		{
			name: "file at the root",
			release: func(t *testing.T) *Release {
				return archiveRelease(t, makeZip(t, map[string]string{"brick_config.yaml": "id: acme:camera\n"}))
			},
			wantErr: "single root folder",
		},
		{
			name: "path traversal",
			release: func(t *testing.T) *Release {
				return archiveRelease(t, makeZip(t, map[string]string{
					"camera/brick_config.yaml": "id: acme:camera\n",
					"camera/../../evil":        "x",
				}))
			},
			wantErr: "invalid entry",
		},
		{
			name: "missing brick_config.yaml",
			release: func(t *testing.T) *Release {
				return archiveRelease(t, makeZip(t, map[string]string{"camera/README.md": "readme"}))
			},
			wantErr: "does not contain a valid brick_config.yaml",
		},
		{
			name: "wrong brick id",
			release: func(t *testing.T) *Release {
				return archiveRelease(t, makeZip(t, map[string]string{"camera/brick_config.yaml": "id: other:camera\n"}))
			},
			wantErr: `declares id "other:camera"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bricksDir := paths.New(t.TempDir())
			_, err := Install(t.Context(), http.DefaultClient, bricksDir, &Brick{ID: "acme:camera"}, tc.release(t), "test")
			require.ErrorContains(t, err, tc.wantErr)

			// Nothing is installed and no temporary folders are left behind.
			entries, err := bricksDir.ReadDir(paths.FilterDirectories())
			require.NoError(t, err)
			require.Empty(t, entries)
		})
	}
}
