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
	yaml "github.com/goccy/go-yaml"
	semver "go.bug.st/relaxed-semver"
)

type InstalledBrick struct {
	Namespace string
	Version   string
	// Brick is read from the brick_config.yaml of the installed version.
	Brick Brick
}

// ListInstalled returns the bricks installed in bricksDir, one entry per version, sorted by
// id and then by version. Folders that are not valid installed bricks are skipped.
func ListInstalled(bricksDir *paths.Path) ([]InstalledBrick, error) {
	if bricksDir.NotExist() {
		return nil, nil
	}
	namespaceDirs, err := bricksDir.ReadDir(paths.FilterDirectories())
	if err != nil {
		return nil, fmt.Errorf("reading installed bricks: %w", err)
	}

	var res []InstalledBrick
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
				brick, err := loadInstalledBrick(brickDir, namespace+":"+name)
				if err != nil {
					slog.Warn("skipping invalid installed brick", "path", brickDir, "error", err)
					continue
				}
				res = append(res, InstalledBrick{Namespace: namespace, Version: version.String(), Brick: brick})
			}
		}
	}

	// The versions are already sorted.
	slices.SortStableFunc(res, func(a, b InstalledBrick) int { return cmp.Compare(a.Brick.ID, b.Brick.ID) })
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

func loadInstalledBrick(brickDir *paths.Path, id string) (Brick, error) {
	data, err := brickDir.Join(brickConfigFileName).ReadFile()
	if err != nil {
		return Brick{}, err
	}
	var brick Brick
	if err := yaml.Unmarshal(data, &brick); err != nil {
		return Brick{}, fmt.Errorf("invalid %s: %w", brickConfigFileName, err)
	}
	if brick.ID != id {
		return Brick{}, fmt.Errorf("%s declares id %q instead of %q", brickConfigFileName, brick.ID, id)
	}
	brick.FullPath = brickDir
	return brick, nil
}
