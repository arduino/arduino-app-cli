// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"cmp"
	"fmt"
	"log/slog"
	"slices"

	"github.com/arduino/go-paths-helper"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/platform"
)

// installedBricks returns the latest version of each installed brick that supports the board.
func (b *BricksIndex) installedBricks() []Brick {
	if b.installedDir == nil {
		return nil
	}
	installed, err := listInstalled(b.platform, b.installedDir)
	if err != nil {
		slog.Warn("cannot read the installed bricks", "error", err)
		return nil
	}
	var res []Brick
	for _, brick := range installed {
		if !brick.SupportsBoard(b.platform) {
			continue
		}
		// The versions of a brick are sorted from the oldest, so the last one wins.
		if n := len(res); n > 0 && res[n-1].ID == brick.ID {
			res[n-1] = brick
		} else {
			res = append(res, brick)
		}
	}
	return res
}

// listInstalled returns the bricks installed in bricksDir, one entry per version, sorted by
// id and then by version. Folders that are not valid installed bricks are skipped.
func listInstalled(platform platform.Platform, bricksDir *paths.Path) ([]Brick, error) {
	if bricksDir.NotExist() {
		return nil, nil
	}
	namespaceDirs, err := bricksDir.ReadDir(paths.FilterDirectories())
	if err != nil {
		return nil, fmt.Errorf("reading installed bricks: %w", err)
	}

	var res []Brick
	for _, namespaceDir := range namespaceDirs {
		namespace := namespaceDir.Base()
		// Skips the temporary folders of install and uninstall.
		if !validBrickIDPart.MatchString(namespace) {
			continue
		}
		nameDirs, err := namespaceDir.ReadDir(paths.FilterDirectories())
		if err != nil {
			return nil, fmt.Errorf("reading installed bricks: %w", err)
		}
		for _, nameDir := range nameDirs {
			name := nameDir.Base()
			if !validBrickIDPart.MatchString(name) {
				continue
			}
			versions, err := installedVersions(nameDir)
			if err != nil {
				return nil, err
			}
			for _, version := range versions {
				brickDir := nameDir.Join(version.String())
				brick, err := loadInstalledBrick(platform, brickDir, namespace, name)
				if err != nil {
					slog.Warn("skipping invalid installed brick", "path", brickDir, "error", err)
					continue
				}
				brick.Version = version.String()
				res = append(res, brick)
			}
		}
	}

	// The versions are already sorted.
	slices.SortStableFunc(res, func(a, b Brick) int { return cmp.Compare(a.ID, b.ID) })
	return res, nil
}

// installedVersions returns the versions installed in the folder of a brick, sorted from the oldest.
// Folders that install could not have created are skipped.
func installedVersions(nameDir *paths.Path) ([]*semver.Version, error) {
	if nameDir.NotExist() {
		return nil, nil
	}
	entries, err := nameDir.ReadDir(paths.FilterDirectories())
	if err != nil {
		return nil, fmt.Errorf("reading installed bricks: %w", err)
	}
	var versions []*semver.Version
	for _, entry := range entries {
		v, err := semver.Parse(entry.Base())
		// semver.Parse accepts an empty string, but a folder name is never empty.
		if err != nil {
			continue
		}
		versions = append(versions, v)
	}
	slices.SortFunc(versions, func(a, b *semver.Version) int { return a.CompareTo(b) })
	return versions, nil
}

func loadInstalledBrick(platform platform.Platform, brickDir *paths.Path, namespace, name string) (Brick, error) {
	brick, err := LoadBrickDir(platform, brickDir, namespace)
	if err != nil {
		return Brick{}, err
	}
	if id := namespace + ":" + name; brick.ID != id {
		return Brick{}, fmt.Errorf("%s declares id %q instead of %q", brickConfigFileName, brick.ID, id)
	}
	return brick, nil
}
