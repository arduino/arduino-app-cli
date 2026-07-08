// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package pipewire

import (
	"fmt"
	"os"
	"path/filepath"
)

// Tracks whether *we* were the one who started PipeWire from cold, so
// TeardownIfUnneeded never stops something another consumer (a real lightdm
// or SSH session) brought up on its own. Lives under the user's own runtime
// dir (tmpfs): a reboot means nothing has started PipeWire yet either, so
// the marker resetting too is correct, not a leak. This directory is used
// instead of a root-owned path under /run because the daemon is
// unprivileged and /run/user/<uid>/ is guaranteed to already exist and be
// writable by it — it's the same directory hosting /run/user/<uid>/bus.
const ownerMarkerSubdir = "arduino-app-cli"

func ownerMarkerPath(uid uint32) string {
	return filepath.Join(runtimeDir(uid), ownerMarkerSubdir, fmt.Sprintf("pipewire-owner-%d", uid))
}

func runtimeDir(uid uint32) string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return dir
	}
	return fmt.Sprintf("/run/user/%d", uid)
}

func markOwned(uid uint32) error {
	path := ownerMarkerPath(uid)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, nil, 0600)
}

func isOwned(uid uint32) (bool, error) {
	_, err := os.Stat(ownerMarkerPath(uid))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func clearOwned(uid uint32) error {
	err := os.Remove(ownerMarkerPath(uid))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
