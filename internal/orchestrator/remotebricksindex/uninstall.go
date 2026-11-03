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
// all of its installed versions. Either all of them are removed or none is.
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

	// The versions are first moved out of the way, so that a failure can put them back and an
	// interrupted removal never leaves a partial brick that looks installed.
	tempDir, err := paths.MkTempDir(bricksDir.String(), uninstallTempPrefix)
	if err != nil {
		return UninstallResult{}, fmt.Errorf("creating temporary directory: %w", err)
	}
	var moved []string
	for _, v := range versions {
		installPath := namespaceDir.Join(name + "@" + v)
		if err := installPath.Rename(tempDir.Join(v)); err != nil {
			for _, m := range moved {
				if rerr := tempDir.Join(m).Rename(namespaceDir.Join(name + "@" + m)); rerr != nil {
					slog.Warn("cannot restore brick", "id", id, "version", m, "error", rerr)
				}
			}
			_ = tempDir.RemoveAll()
			return UninstallResult{}, fmt.Errorf("uninstalling brick %s@%s: %w", id, v, err)
		}
		moved = append(moved, v)
	}
	fatomic.Sync()

	// The brick is already uninstalled: what is left is only garbage.
	if err := tempDir.RemoveAll(); err != nil {
		slog.Warn("cannot remove uninstalled brick files", "path", tempDir, "error", err)
	}
	// Fails, as expected, if other bricks of the namespace are installed.
	_ = os.Remove(namespaceDir.String())

	return UninstallResult{Versions: versions}, nil
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
