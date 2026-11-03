// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package remotebricksindex

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/arduino/go-paths-helper"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/fatomic"
)

type UninstallResult struct {
	// Versions are the uninstalled versions, empty if the brick was not installed.
	Versions []string
}

// Uninstall removes the given version of the brick from bricksDir or, if version is empty,
// all of its installed versions. On error, the result contains the versions removed so far.
func Uninstall(ctx context.Context, bricksDir *paths.Path, id, version string) (UninstallResult, error) {
	namespace, name, err := splitBrickID(id)
	if err != nil {
		return UninstallResult{}, err
	}
	if version != "" {
		if _, err := InstallPath(bricksDir, id, version); err != nil {
			return UninstallResult{}, err
		}
	}
	if bricksDir.NotExist() {
		return UninstallResult{}, nil
	}

	unlock, err := lockBricksDir(ctx, bricksDir)
	if err != nil {
		return UninstallResult{}, err
	}
	defer unlock()

	namespaceDir := bricksDir.Join(namespace)
	versions, err := installedVersions(namespaceDir, name)
	if err != nil {
		return UninstallResult{}, err
	}
	if version != "" {
		if !slices.Contains(versions, version) {
			return UninstallResult{}, nil
		}
		versions = []string{version}
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
		// Fails, as expected, if other bricks of the namespace are installed.
		_ = os.Remove(namespaceDir.String())
	}()

	var res UninstallResult
	for _, v := range versions {
		if err := namespaceDir.Join(name + "@" + v).Rename(tempDir.Join(v)); err != nil {
			return res, fmt.Errorf("uninstalling brick %s@%s: %w", id, v, err)
		}
		res.Versions = append(res.Versions, v)
	}
	fatomic.Sync()
	return res, nil
}

// installedVersions returns the installed versions of the brick, sorted from the oldest.
func installedVersions(namespaceDir *paths.Path, name string) ([]string, error) {
	if namespaceDir.NotExist() {
		return nil, nil
	}
	entries, err := namespaceDir.ReadDir(paths.FilterDirectories())
	if err != nil {
		return nil, fmt.Errorf("reading installed bricks: %w", err)
	}
	var versions []*semver.Version
	for _, entry := range entries {
		v, ok := strings.CutPrefix(entry.Base(), name+"@")
		if !ok || v == "" {
			continue
		}
		// Folders that install could not have created are left alone.
		parsed, err := semver.Parse(v)
		if err != nil {
			continue
		}
		versions = append(versions, parsed)
	}
	slices.SortFunc(versions, func(a, b *semver.Version) int { return a.CompareTo(b) })
	res := make([]string, len(versions))
	for i, v := range versions {
		res[i] = v.String()
	}
	return res, nil
}
