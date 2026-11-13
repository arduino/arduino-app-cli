// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"fmt"
	"log/slog"
	"slices"

	"github.com/arduino/go-paths-helper"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/platform"
)

// installedBricks returns, for each installed brick, its latest version that supports the board,
// sorted by id.
func (b *BricksIndex) installedBricks() []Brick {
	var res []Brick
	for _, id := range b.installedIDs() {
		if brick, found := b.findInstalledBrick(id); found {
			res = append(res, brick)
		}
	}
	return res
}

// AllInstalledBricks returns every installed version of every installed brick, whatever the
// boards it supports, sorted by id and version. Versions that are not valid installed bricks are skipped.
func (b *BricksIndex) AllInstalledBricks() []Brick {
	var res []Brick
	for _, id := range b.installedIDs() {
		namespace, name, _ := splitBrickID(id) // installedIDs returns valid ids only
		nameDir := b.installedDir.Join(namespace, name)
		versions, err := installedVersions(nameDir)
		if err != nil {
			slog.Warn("cannot read the installed bricks", "error", err)
			continue
		}
		for _, version := range versions {
			brickDir := nameDir.Join(version.String())
			brick, err := loadInstalledBrick(b.platform, brickDir, namespace, name)
			if err != nil {
				slog.Warn("skipping invalid installed brick", "path", brickDir, "error", err)
				continue
			}
			brick.Version = version.String()
			res = append(res, brick)
		}
	}
	return res
}

// installedIDs returns the ids of the installed bricks, sorted.
func (b *BricksIndex) installedIDs() []string {
	if b.installedDir == nil || b.installedDir.NotExist() {
		return nil
	}
	namespaceDirs, err := b.installedDir.ReadDir(paths.FilterDirectories())
	if err != nil {
		slog.Warn("cannot read the installed bricks", "error", err)
		return nil
	}
	var res []string
	for _, namespaceDir := range namespaceDirs {
		nameDirs, err := namespaceDir.ReadDir(paths.FilterDirectories())
		if err != nil {
			slog.Warn("cannot read the installed bricks", "error", err)
			continue
		}
		for _, nameDir := range nameDirs {
			// The temporary folders of install and uninstall are not valid ids, so they are skipped.
			id := namespaceDir.Base() + ":" + nameDir.Base()
			if _, _, err := splitBrickID(id); err == nil {
				res = append(res, id)
			}
		}
	}
	slices.Sort(res)
	return res
}

// findInstalledBrick returns the latest installed version of the brick that supports the board.
// Versions that are not valid installed bricks are skipped.
func (b *BricksIndex) findInstalledBrick(id string) (Brick, bool) {
	namespace, name, err := splitBrickID(id)
	if b.installedDir == nil || err != nil {
		return Brick{}, false
	}
	nameDir := b.installedDir.Join(namespace, name)
	versions, err := installedVersions(nameDir)
	if err != nil {
		slog.Warn("cannot read the installed bricks", "error", err)
		return Brick{}, false
	}
	for _, version := range slices.Backward(versions) {
		brickDir := nameDir.Join(version.String())
		brick, err := loadInstalledBrick(b.platform, brickDir, namespace, name)
		if err != nil {
			slog.Warn("skipping invalid installed brick", "path", brickDir, "error", err)
			continue
		}
		if brick.SupportsBoard(b.platform) {
			brick.Version = version.String()
			return brick, true
		}
	}
	return Brick{}, false
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

const (
	releaseSrcDir      = "src"
	releaseDocsDir     = "docs"
	releaseExamplesDir = "examples"
)

func loadInstalledBrick(platform platform.Platform, brickDir *paths.Path, namespace, name string) (Brick, error) {
	brick, err := LoadBrickDir(platform, brickDir.Join(releaseSrcDir), namespace)
	if err != nil {
		return Brick{}, err
	}
	if id := namespace + ":" + name; brick.ID != id {
		return Brick{}, fmt.Errorf("%s declares id %q instead of %q", brickConfigFileName, brick.ID, id)
	}
	brick.ExamplesPath = brickDir.Join(releaseExamplesDir)
	brick.DocsAPIPath = brickDir.Join(releaseDocsDir, "API.md")
	return brick, nil
}
