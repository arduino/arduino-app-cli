// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package dockerhelper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureBindDirs(t *testing.T) {
	t.Run("creates a missing bind directory", func(t *testing.T) {
		root := t.TempDir()
		hostPath := filepath.Join(root, "models", "llamacpp")

		require.NoError(t, ensureBindDirs([]string{hostPath + ":/models"}))
		info, err := os.Stat(hostPath)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("accepts an existing writable directory", func(t *testing.T) {
		hostPath := t.TempDir()
		require.NoError(t, ensureBindDirs([]string{hostPath + ":/models"}))
	})

	// The regression: the daemon used to log a warning and carry on, leaving
	// Docker to create the path as root and the container unable to write to it.
	t.Run("refuses a directory it cannot create", func(t *testing.T) {
		requireNotRoot(t)
		root := t.TempDir()
		parent := filepath.Join(root, "models")
		require.NoError(t, os.Mkdir(parent, 0o555))
		t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })

		err := ensureBindDirs([]string{filepath.Join(parent, "llamacpp") + ":/models"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot create bind mount directory")
		assert.Contains(t, err.Error(), "owned by")
	})

	t.Run("refuses an existing directory it cannot write to", func(t *testing.T) {
		requireNotRoot(t)
		hostPath := t.TempDir()
		require.NoError(t, os.Chmod(hostPath, 0o555))
		t.Cleanup(func() { _ = os.Chmod(hostPath, 0o755) })

		err := ensureBindDirs([]string{hostPath + ":/models"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not writable by the container user")
	})

	t.Run("allows an unwritable directory when the mount is read-only", func(t *testing.T) {
		requireNotRoot(t)
		hostPath := t.TempDir()
		require.NoError(t, os.Chmod(hostPath, 0o555))
		t.Cleanup(func() { _ = os.Chmod(hostPath, 0o755) })

		require.NoError(t, ensureBindDirs([]string{hostPath + ":/models:ro"}))
	})

	t.Run("rejects a malformed bind", func(t *testing.T) {
		for _, bind := range []string{"", "/no-container-path", ":/models"} {
			err := ensureBindDirs([]string{bind})
			require.Error(t, err, "bind %q should be rejected", bind)
			assert.Contains(t, err.Error(), "malformed bind mount")
		}
	})
}

func TestIsReadOnlyBind(t *testing.T) {
	for _, tc := range []struct {
		mountSpec string
		want      bool
	}{
		{"/models", false},
		{"/models:ro", true},
		{"/models:rw", false},
		{"/models:ro,z", true},
		{"/models:z,ro", true},
		{"/models:rox", false},
	} {
		assert.Equal(t, tc.want, isReadOnlyBind(tc.mountSpec), "mountSpec %q", tc.mountSpec)
	}
}

// requireNotRoot skips a test that depends on permission checks having teeth,
// since root bypasses them.
func requireNotRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission checks do not apply")
	}
}
