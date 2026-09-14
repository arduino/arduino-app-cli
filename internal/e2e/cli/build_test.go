// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/e2e"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

// TestAppBuild builds a release out of a new app. The python environment is built in the
// runner image, which is arm64 only, so a docker daemon is required as well.
func TestAppBuild(t *testing.T) {
	if runtime.GOARCH != "arm64" {
		t.Skipf("Skipping test: requires arm64 architecture, currently running on %s", runtime.GOARCH)
	}

	cli := e2e.NewArduinoAppCLI(t, e2e.WithBoardName("unoq"))
	t.Cleanup(cli.CleanUp)

	tests := []struct {
		name string
		// appName is the app the case builds, one per case: an app is built once.
		appName string
		// newArgs are the flags app new is given, so a case states the app it needs.
		newArgs []string
		// wantBricks is what the manifest must state, and so what the frozen compose set
		// must hold a brick compose for.
		wantBricks []string
		// requirements is the python dependency the app declares, so the case proves the
		// venv the release ships holds it already.
		requirements string
		// archiveName is the file --output is given, so a case proves the release folder
		// follows it. Empty leaves the naming to the cli.
		archiveName string
	}{
		{
			name:        "an app with no sketch and no bricks",
			appName:     "plain-app",
			newArgs:     []string{"--no-sketch"},
			archiveName: "my-release" + orchestrator.ReleaseArchiveExt,
		},
		{
			name:       "an app with a brick",
			appName:    "brick-app",
			newArgs:    []string{"--no-sketch", "--bricks", "arduino:dbstorage_tsstore"},
			wantBricks: []string{"arduino:dbstorage_tsstore"},
		},
		{
			name:         "an app with a python dependency",
			appName:      "deps-app",
			newArgs:      []string{"--no-sketch"},
			requirements: "six==1.17.0\n",
		},
		// TODO: an app with a sketch, once the firmware it flashes ships in the release.
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// The runner image is pulled before the venv is built, so a build is not quick.
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Minute)
			defer cancel()

			newArgs := append([]string{"app", "new", test.appName}, test.newArgs...)
			stdout, stderr, err := cli.Run(ctx, newArgs...)
			require.NoError(t, err, "stdout: %s\nstderr: %s", stdout, stderr)

			appDir := cli.AppsDir().Join(test.appName)
			if test.requirements != "" {
				require.NoError(t, appDir.Join("python", "requirements.txt").WriteFile([]byte(test.requirements)))
			}
			asAuthored := appEntries(t, appDir)

			outputDir := paths.New(t.TempDir())
			output := outputDir
			if test.archiveName != "" {
				output = outputDir.Join(test.archiveName)
			}
			buildStart := time.Now().UTC().Truncate(time.Second)
			stdout, stderr, err = cli.Run(ctx, "app", "build", appDir.String(), "--output", output.String())
			require.NoError(t, err, "stdout: %s\nstderr: %s", stdout, stderr)

			// The build writes nothing in the app: it is staged and built outside of it,
			// and the runner gets the staged copy read-only.
			assert.Equal(t, asAuthored, appEntries(t, appDir))

			archives, err := outputDir.ReadDir()
			require.NoError(t, err)
			archives.FilterSuffix(orchestrator.ReleaseArchiveExt)
			require.Len(t, archives, 1, "stdout: %s\nstderr: %s", stdout, stderr)
			archivePath := archives[0]
			releaseName := strings.TrimSuffix(archivePath.Base(), orchestrator.ReleaseArchiveExt)

			names, manifest := readRelease(t, archivePath)

			assert.Equal(t, orchestrator.ReleaseManifestSchema, manifest.Schema)
			assert.Equal(t, test.appName, manifest.Name)
			// The board of platform.json, which is the one running the build.
			assert.Equal(t, "unoq", manifest.Target)
			// Dated in UTC, and the same instant names the release.
			_, offset := manifest.CreatedAt.Zone()
			assert.Zero(t, offset)
			assert.WithinRange(t, manifest.CreatedAt, buildStart, time.Now().UTC())
			wantName := test.appName + "-" + manifest.CreatedAt.Format("20060102-150405") + "-unoq"
			if test.archiveName != "" {
				wantName = strings.TrimSuffix(test.archiveName, orchestrator.ReleaseArchiveExt)
			}
			assert.Equal(t, wantName, releaseName)

			bricks := make([]string, 0, len(manifest.Bricks))
			for _, brick := range manifest.Bricks {
				bricks = append(bricks, brick.ID)
			}
			assert.ElementsMatch(t, test.wantBricks, bricks)

			// The manifest is the first entry after the folder the archive is rooted at,
			// so a reader gets the release facts from the first block.
			require.Greater(t, len(names), 2)
			assert.Equal(t, releaseName, names[0])
			assert.Equal(t, releaseName+"/"+orchestrator.ReleaseManifestFileName, names[1])

			// The app as authored, the frozen compose set and the brick index the board reads.
			assert.Contains(t, names, releaseName+"/src/app.yaml")
			assert.Contains(t, names, releaseName+"/prebuild/"+app.MainTemplateFileName)
			assert.Contains(t, names, releaseName+"/prebuild/bricks-list.yaml")

			ships := func(prefix string) bool {
				return slices.ContainsFunc(names, func(name string) bool { return strings.HasPrefix(name, prefix) })
			}
			// The venv is what the build is for: the board installs nothing.
			assert.True(t, ships(releaseName+"/prebuild/.venv/"), "the release ships no python environment")

			// A brick with a container brings a compose to include, and the overrides of
			// the services it declares. An app with none has neither.
			withContainer := len(test.wantBricks) > 0
			assert.Equal(t, withContainer, ships(releaseName+"/prebuild/compose/"),
				"the frozen compose set does not match the bricks of the app")
			assert.Equal(t, withContainer, ships(releaseName+"/prebuild/"+app.OverrideTemplateFileName),
				"the overrides do not match the services of the app")

			// A declared dependency is installed at build, and the marker run.sh reads to
			// skip the install ships next to the venv.
			holdsDependency := slices.ContainsFunc(names, func(name string) bool {
				return strings.Contains(name, "/site-packages/six-1.17.0.dist-info")
			})
			assert.Equal(t, test.requirements != "", holdsDependency,
				"the venv does not match the requirements of the app")
			assert.Equal(t, test.requirements != "", ships(releaseName+"/prebuild/installed_requirements.txt"),
				"the install marker does not match the requirements of the app")

			// No data dir ships, and the app .cache is resolved anew by the build.
			for _, name := range names {
				assert.NotContains(t, name, "/data/")
				assert.NotContains(t, name, "/src/.cache")
			}
		})
	}
}

// appEntries is every path of the app folder, relative and sorted: a build that writes
// in the app is a build that ships something else than what was authored.
func appEntries(t *testing.T, appDir *paths.Path) []string {
	t.Helper()

	entries, err := appDir.ReadDirRecursive()
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		relPath, err := entry.RelFrom(appDir)
		require.NoError(t, err)
		names = append(names, relPath.String())
	}
	slices.Sort(names)
	return names
}

// readRelease is the entry names of the archive, in the order they are written, and the
// manifest it ships.
func readRelease(t *testing.T, archivePath *paths.Path) ([]string, orchestrator.ReleaseManifest) {
	t.Helper()

	file, err := archivePath.Open()
	require.NoError(t, err)
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	require.NoError(t, err)
	defer gzipReader.Close()

	var names []string
	var manifest orchestrator.ReleaseManifest
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return names, manifest
		}
		require.NoError(t, err)
		names = append(names, header.Name)

		if strings.HasSuffix(header.Name, "/"+orchestrator.ReleaseManifestFileName) {
			content, err := io.ReadAll(tarReader)
			require.NoError(t, err)
			require.NoError(t, yaml.Unmarshal(content, &manifest))
		}
	}
}
