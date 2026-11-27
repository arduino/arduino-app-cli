// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

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

func TestValidateBrickID(t *testing.T) {
	for _, id := range []string{"acme:camera", "acme_2:my_camera2"} {
		require.NoError(t, ValidateBrickID(id), id)
	}
	for _, id := range []string{"", "camera", ":camera", "acme:", "acme:Camera", "acme:my-camera", "acme:2camera", "acme:_camera", "acme.io:camera", "acme:camera/..", "../acme:camera", "acme:camera@1.0.0"} {
		require.ErrorContains(t, ValidateBrickID(id), "invalid brick id", id)
	}
}

func TestParseBrickVersion(t *testing.T) {
	version, err := ParseBrickVersion("1.2.0")
	require.NoError(t, err)
	require.Equal(t, "1.2.0", version.String())

	_, err = ParseBrickVersion("")
	require.ErrorContains(t, err, "missing version")
	for _, v := range []string{"v1", "1.0.0/..", "latest"} {
		_, err := ParseBrickVersion(v)
		require.Error(t, err, v)
	}
}

func TestFindBrickRelease(t *testing.T) {
	release := func(version string, boards ...string) RemoteRelease {
		return RemoteRelease{Version: semver.MustParse(version), SupportedBoards: boards}
	}
	index := &RemoteIndex{Bricks: []RemoteBrick{
		{ID: "acme:camera", Releases: []RemoteRelease{release("1.10.0", "ventunoq"), release("1.2.0"), release("1.9.0", "unoq", "ventunoq")}},
		{ID: "acme:audio", Releases: []RemoteRelease{release("1.0.0", "ventunoq")}},
		{ID: "acme:empty"},
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
		{"latest release, compared as semver", "acme:camera", "", platform.Platform{BoardName: "ventunoq"}, "1.10.0", nil},
		// An older release that supports the board is not chosen instead.
		{"latest release not supported", "acme:camera", "", unoq, "", ErrUnsupportedBoard},
		{"any board without a board name", "acme:camera", "", platform.Platform{}, "1.10.0", nil},
		{"given version", "acme:camera", "1.2.0", unoq, "1.2.0", nil},
		{"given version, compared as semver", "acme:camera", "1.2", unoq, "1.2.0", nil},
		{"given version not supported", "acme:camera", "1.10.0", unoq, "", ErrUnsupportedBoard},
		{"no release supported", "acme:audio", "", unoq, "", ErrUnsupportedBoard},
		{"missing version", "acme:camera", "9.9.9", unoq, "", ErrReleaseNotFound},
		{"no releases", "acme:empty", "", unoq, "", ErrReleaseNotFound},
		{"missing brick", "acme:missing", "", unoq, "", ErrBrickNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var version *semver.Version
			if tt.version != "" {
				version = semver.MustParse(tt.version)
			}
			brick, release, err := index.FindBrickRelease(tt.id, version, tt.platform)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, brick)
				require.Nil(t, release)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.id, brick.ID)
			require.Equal(t, tt.want, release.Version.String())
		})
	}

	// The resolution of the bricks of an app falls back to the latest release that supports the board.
	camera, err := index.FindBrick("acme:camera")
	require.NoError(t, err)
	latestSupported, err := camera.FindRelease(nil, unoq)
	require.NoError(t, err)
	require.Equal(t, "1.9.0", latestSupported.Version.String())

	releases := camera.SupportedReleases(unoq)
	versions := make([]string, 0, len(releases))
	for _, r := range releases {
		versions = append(versions, r.Version.String())
	}
	require.Equal(t, []string{"1.2.0", "1.9.0"}, versions)
}
func TestInstall(t *testing.T) {
	const image = "ghcr.io/acme/camera:1.0@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	files := map[string]string{
		"src/camera/brick_config.yaml":  "id: acme:camera\n",
		"src/camera/brick_compose.yaml": "services:\n  camera:\n    image: " + image + "\n",
		"src/camera/README.md":          "readme",
		"docs/API.md":                   "api",
		"examples/basic/app.yaml":       "name: basic",
		"LICENSE":                       "license",
	}
	for _, root := range []string{"", "camera-1.0.0/"} {
		t.Run("root folder "+root, func(t *testing.T) {
			bricksDir := paths.New(t.TempDir())
			archive := map[string]string{}
			for name, content := range files {
				archive[root+name] = content
			}
			release := archiveRelease(t, makeZip(t, archive))

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
			res, err := Install(t.Context(), DownloadConfig("test"), bricksDir, cameraBrick, release, platform.Platform{}, progress, prepare)
			require.NoError(t, err)
			require.False(t, res.AlreadyInstalled)
			require.Equal(t, []string{"acme:camera@1.0.0"}, prepared)
			require.Equal(t, release.Size, downloaded)
			require.Equal(t, release.Size, total)
			require.Equal(t, bricksDir.Join("acme", "camera", "1.0.0").String(), res.Path.String())
			for name := range files {
				require.FileExists(t, res.Path.Join(name).String())
			}

			res, err = Install(t.Context(), DownloadConfig("test"), bricksDir, cameraBrick, release, platform.Platform{}, nil, nil)
			require.NoError(t, err)
			require.True(t, res.AlreadyInstalled)

			// No temporary folders are left behind.
			entries, err := bricksDir.ReadDir(paths.FilterDirectories())
			require.NoError(t, err)
			require.Len(t, entries, 1)
		})
	}
}

func TestInstallReportsDownloadOncePerPercent(t *testing.T) {
	// Random content does not compress. The archive is served slowly and the downloader polled
	// much more often, so that many polls see the same percentage.
	content := make([]byte, 1<<20)
	_, err := rand.Read(content)
	require.NoError(t, err)
	archive := makeZip(t, map[string]string{
		"src/camera/brick_config.yaml": "id: acme:camera\n",
		"src/camera/data.bin":          string(content),
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(archive)))
		if r.Method == http.MethodHead {
			return
		}
		const parts = 20
		for i := range parts {
			_, _ = w.Write(archive[i*len(archive)/parts : (i+1)*len(archive)/parts])
			w.(http.Flusher).Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	t.Cleanup(srv.Close)
	release := archiveReleaseAt(t, archive, srv.URL+"/camera.zip")
	config := DownloadConfig("test")
	config.PollInterval = time.Millisecond

	var percents []int64
	progress := func(downloaded, total int64) {
		require.Equal(t, release.Size, total)
		percents = append(percents, downloaded*100/total)
	}
	_, err = Install(t.Context(), config, paths.New(t.TempDir()), cameraBrick, release, platform.Platform{}, progress, nil)
	require.NoError(t, err)

	require.Greater(t, len(percents), 1, "the download must be reported more than once to check it")
	require.Equal(t, int64(100), percents[len(percents)-1])
	for i := 1; i < len(percents); i++ {
		require.Greater(t, percents[i], percents[i-1], "each percent is reported once: %v", percents)
	}
}

func TestInstallConcurrent(t *testing.T) {
	bricksDir := paths.New(t.TempDir())
	release := archiveRelease(t, makeZip(t, map[string]string{"src/camera/brick_config.yaml": "id: acme:camera\n"}))
	brickPath := bricksDir.Join("acme", "camera", "1.0.0")
	// Another install of the same release finishes while this one is preparing.
	prepare := func(Brick) error {
		require.NoError(t, brickPath.Join("src").MkdirAll())
		return nil
	}
	res, err := Install(t.Context(), DownloadConfig("test"), bricksDir, cameraBrick, release, platform.Platform{}, nil, prepare)
	require.NoError(t, err)
	require.True(t, res.AlreadyInstalled)
	require.Equal(t, brickPath.String(), res.Path.String())
}

func TestInstallFailures(t *testing.T) {
	validArchive := makeZip(t, map[string]string{"camera/src/camera/brick_config.yaml": "id: acme:camera\n"})
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
			name: "multiple root folders",
			release: func(t *testing.T) *RemoteRelease {
				return archiveRelease(t, makeZip(t, map[string]string{
					"camera/src/camera/brick_config.yaml": "id: acme:camera\n",
					"other/file":                          "x",
				}))
			},
			wantErr: "does not contain a src folder",
		},
		{
			name: "no src folder",
			release: func(t *testing.T) *RemoteRelease {
				return archiveRelease(t, makeZip(t, map[string]string{"brick_config.yaml": "id: acme:camera\n"}))
			},
			wantErr: "does not contain a src folder",
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
			_, err := Install(t.Context(), DownloadConfig("test"), bricksDir, cameraBrick, tc.release(t), platform.Platform{BoardName: "unoq"}, nil, tc.prepare)
			require.ErrorContains(t, err, tc.wantErr)

			// Nothing is installed and no temporary folders are left behind.
			entries, err := bricksDir.ReadDir(paths.FilterDirectories())
			require.NoError(t, err)
			require.Empty(t, entries)
		})
	}
}
