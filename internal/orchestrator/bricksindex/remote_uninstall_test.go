// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
)

// installedBricks creates the given folders, in the form namespace/name@version.
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
	bricksDir := installedBricks(t, "acme/camera@1.0.0", "acme/camera@1.2.0")

	res, err := Uninstall(t.Context(), bricksDir, "acme:camera", "1.0.0")
	require.NoError(t, err)
	require.Equal(t, []string{"1.0.0"}, res.Versions)
	require.Equal(t, []string{"camera@1.2.0"}, dirNames(t, bricksDir.Join("acme")))

	res, err = Uninstall(t.Context(), bricksDir, "acme:camera", "1.0.0")
	require.NoError(t, err)
	require.Empty(t, res.Versions)

	// Removing the last brick of the namespace removes the namespace folder too.
	_, err = Uninstall(t.Context(), bricksDir, "acme:camera", "1.2.0")
	require.NoError(t, err)
	require.Empty(t, dirNames(t, bricksDir))
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
	require.ElementsMatch(t, []string{"camera2@1.0.0", "camera@notes"}, dirNames(t, bricksDir.Join("acme")))
	// No temporary folders are left behind.
	require.Equal(t, []string{"acme"}, dirNames(t, bricksDir))
}

func TestUninstallNotInstalled(t *testing.T) {
	res, err := Uninstall(t.Context(), paths.New(t.TempDir(), "missing"), "acme:camera", "")
	require.NoError(t, err)
	require.Empty(t, res.Versions)

	res, err = Uninstall(t.Context(), installedBricks(t), "acme:camera", "")
	require.NoError(t, err)
	require.Empty(t, res.Versions)
}
