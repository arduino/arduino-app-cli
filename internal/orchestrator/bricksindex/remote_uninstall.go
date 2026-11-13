// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"

	"github.com/arduino/go-paths-helper"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/fatomic"
)

type UninstallResult struct {
	// Versions are the uninstalled versions, empty if the brick was not installed.
	Versions []string
}

// Uninstall removes the given version of the brick from bricksDir or, if version is nil,
// all of its installed versions. On error, the result contains the versions removed so far.
// onWait, if not nil, is called when the uninstall has to wait for another install or uninstall.
func Uninstall(ctx context.Context, bricksDir *paths.Path, id string, version *semver.Version, onWait func()) (UninstallResult, error) {
	namespace, name, err := splitBrickID(id)
	if err != nil {
		return UninstallResult{}, err
	}
	if bricksDir.NotExist() {
		return UninstallResult{}, nil
	}

	unlock, err := lockBricksDir(ctx, bricksDir, onWait)
	if err != nil {
		return UninstallResult{}, err
	}
	defer unlock()

	namespaceDir := bricksDir.Join(namespace)
	nameDir := namespaceDir.Join(name)
	versions, err := installedVersions(nameDir)
	if err != nil {
		return UninstallResult{}, err
	}
	if version != nil {
		versions = slices.DeleteFunc(versions, func(v *semver.Version) bool { return !v.Equal(version) })
	}
	if len(versions) == 0 {
		return UninstallResult{}, nil
	}

	// Each version is moved out of the way with an atomic rename before being deleted, so that an
	// interrupted removal never leaves a partial brick that looks installed.
	tempDir, err := paths.MkTempDir(bricksDir.String(), uninstallTempPrefix)
	if err != nil {
		return UninstallResult{}, fmt.Errorf("creating temporary directory: %w", err)
	}
	defer func() {
		if err := tempDir.RemoveAll(); err != nil {
			slog.Warn("cannot remove uninstalled brick files", "path", tempDir, "error", err)
		}
		// They fail, as expected, if other versions or other bricks of the namespace are installed.
		_ = os.Remove(nameDir.String())
		_ = os.Remove(namespaceDir.String())
	}()

	var res UninstallResult
	for _, v := range versions {
		if err := nameDir.Join(v.String()).Rename(tempDir.Join(v.String())); err != nil {
			return res, fmt.Errorf("uninstalling brick %s@%s: %w", id, v, err)
		}
		res.Versions = append(res.Versions, v.String())
	}
	fatomic.Sync()
	return res, nil
}
