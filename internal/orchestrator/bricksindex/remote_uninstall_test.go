// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/gofrs/flock"
	"github.com/stretchr/testify/require"
	semver "go.bug.st/relaxed-semver"
)

// installedBricks creates the given folders, in the form namespace/name/version.
func installedBricks(t *testing.T, folders ...string) *paths.Path {
	t.Helper()
	bricksDir := paths.New(t.TempDir())
	for _, f := range folders {
		require.NoError(t, bricksDir.Join(f).MkdirAll())
	}
	return bricksDir
}

func dirNames(t *testing.T, dir *paths.Path) []string {
	t.Helper()
	entries, err := dir.ReadDir(paths.FilterDirectories())
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Base())
	}
	return names
}

func TestUninstallVersion(t *testing.T) {
	bricksDir := installedBricks(t, "acme/camera/1.0.0", "acme/camera/1.2.0")

	res, err := Uninstall(t.Context(), bricksDir, "acme:camera", semver.MustParse("1.0.0"), nil)
	require.NoError(t, err)
	require.Equal(t, []string{"1.0.0"}, res.Versions)
	require.Equal(t, []string{"1.2.0"}, dirNames(t, bricksDir.Join("acme", "camera")))

	res, err = Uninstall(t.Context(), bricksDir, "acme:camera", semver.MustParse("1.0.0"), nil)
	require.NoError(t, err)
	require.Empty(t, res.Versions)

	// The version is compared as semver.
	res, err = Uninstall(t.Context(), bricksDir, "acme:camera", semver.MustParse("1.2"), nil)
	require.NoError(t, err)
	require.Equal(t, []string{"1.2.0"}, res.Versions)
	// Removing the last version of the last brick of the namespace removes its folders too.
	require.Empty(t, dirNames(t, bricksDir))
}

func TestUninstallAllVersions(t *testing.T) {
	bricksDir := installedBricks(t,
		"acme/camera/1.10.0", "acme/camera/1.2.0",
		"acme/camera2/1.0.0", // another brick whose name starts the same
		"acme/camera/notes",  // not created by install
	)

	res, err := Uninstall(t.Context(), bricksDir, "acme:camera", nil, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"1.2.0", "1.10.0"}, res.Versions)
	require.Equal(t, []string{"notes"}, dirNames(t, bricksDir.Join("acme", "camera")))
	require.ElementsMatch(t, []string{"camera", "camera2"}, dirNames(t, bricksDir.Join("acme")))
	// No temporary folders are left behind.
	require.Equal(t, []string{"acme"}, dirNames(t, bricksDir))
}

func TestUninstallNotInstalled(t *testing.T) {
	res, err := Uninstall(t.Context(), paths.New(t.TempDir(), "missing"), "acme:camera", nil, nil)
	require.NoError(t, err)
	require.Empty(t, res.Versions)

	res, err = Uninstall(t.Context(), installedBricks(t), "acme:camera", nil, nil)
	require.NoError(t, err)
	require.Empty(t, res.Versions)
}

func TestLockBricksDir(t *testing.T) {
	bricksDir := installedBricks(t, "acme/camera/1.0.0", ".install-123/x", ".uninstall-456/camera/1.0.0")

	// Another operation holds the lock: onWait is called, and the lock is taken once released.
	other := flock.New(bricksDir.Join(installLockFileName).String())
	locked, err := other.TryLock()
	require.NoError(t, err)
	require.True(t, locked)
	waited := false
	unlock, err := lockBricksDir(t.Context(), bricksDir, func() {
		waited = true
		require.NoError(t, other.Unlock())
	})
	require.NoError(t, err)
	require.True(t, waited)
	// The temporary folders left by interrupted operations are removed.
	require.Equal(t, []string{"acme"}, dirNames(t, bricksDir))
	unlock()

	waited = false
	unlock, err = lockBricksDir(t.Context(), bricksDir, func() { waited = true })
	require.NoError(t, err)
	require.False(t, waited)
	unlock()
}
