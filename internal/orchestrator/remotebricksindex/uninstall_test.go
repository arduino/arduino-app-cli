// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package remotebricksindex

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
)

// installedBricks creates the folders of the given installed bricks, in the form namespace/name@version.
func installedBricks(t *testing.T, folders ...string) *paths.Path {
	t.Helper()
	bricksDir := paths.New(t.TempDir())
	for _, f := range folders {
		dir := bricksDir.Join(f)
		require.NoError(t, dir.MkdirAll())
		require.NoError(t, dir.Join(brickConfigFileName).WriteFile([]byte("id: x\n")))
	}
	return bricksDir
}

func TestUninstallVersion(t *testing.T) {
	bricksDir := installedBricks(t, "acme/camera@1.0.0", "acme/camera@1.2.0")

	res, err := Uninstall(t.Context(), bricksDir, "acme:camera", "1.0.0")
	require.NoError(t, err)
	require.Equal(t, []string{"1.0.0"}, res.Versions)
	require.NoDirExists(t, bricksDir.Join("acme", "camera@1.0.0").String())
	require.DirExists(t, bricksDir.Join("acme", "camera@1.2.0").String())

	res, err = Uninstall(t.Context(), bricksDir, "acme:camera", "1.0.0")
	require.NoError(t, err)
	require.Empty(t, res.Versions)
}

func TestUninstallAllVersions(t *testing.T) {
	bricksDir := installedBricks(t,
		"acme/camera@1.10.0", "acme/camera@1.2.0",
		"acme/camera2@1.0.0", // another brick whose name starts the same
		"acme/camera@notes",  // not created by install
	)

	res, err := Uninstall(t.Context(), bricksDir, "acme:camera", "")
	require.NoError(t, err)
	require.Equal(t, []string{"1.2.0", "1.10.0"}, res.Versions)

	entries, err := bricksDir.Join("acme").ReadDir()
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"camera2@1.0.0", "camera@notes"}, []string{entries[0].Base(), entries[1].Base()})

	// No temporary folders are left behind.
	entries, err = bricksDir.ReadDir(paths.FilterDirectories())
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestUninstallRemovesEmptyNamespace(t *testing.T) {
	bricksDir := installedBricks(t, "acme/camera@1.0.0")

	_, err := Uninstall(t.Context(), bricksDir, "acme:camera", "")
	require.NoError(t, err)
	require.NoDirExists(t, bricksDir.Join("acme").String())
}

func TestUninstallNotInstalled(t *testing.T) {
	res, err := Uninstall(t.Context(), paths.New(t.TempDir(), "missing"), "acme:camera", "")
	require.NoError(t, err)
	require.Empty(t, res.Versions)

	res, err = Uninstall(t.Context(), installedBricks(t), "acme:camera", "")
	require.NoError(t, err)
	require.Empty(t, res.Versions)
}

func TestUninstallRejectsUnsafeReferences(t *testing.T) {
	bricksDir := installedBricks(t)
	_, err := Uninstall(t.Context(), bricksDir, "acme:..", "")
	require.Error(t, err)
	_, err = Uninstall(t.Context(), bricksDir, "acme:camera", "../1.0.0")
	require.Error(t, err)
}
