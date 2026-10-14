// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !windows

package fatomic

import (
	"os"
	"path/filepath"

	"github.com/google/renameio/v2"
)

// WriteFile replaces filename atomically, and syncs the file and its directory to disk.
func WriteFile(filename string, data []byte, perm os.FileMode, opts ...renameio.Option) error {
	if err := renameio.WriteFile(filename, data, perm); err != nil {
		return err
	}
	// renameio syncs the file but not the directory, so the rename can be lost.
	dir, err := os.Open(filepath.Dir(filename))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
