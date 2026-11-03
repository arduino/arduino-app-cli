// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package remotebricksindex

import (
	"cmp"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

type InstalledBrick struct {
	Namespace string
	Version   string
	// Brick is read from the brick_config.yaml of the installed version.
	Brick bricksindex.Brick
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
		if !validFolderName.MatchString(namespace) {
			continue
		}
		brickDirs, err := namespaceDir.ReadDir(paths.FilterDirectories())
		if err != nil {
			return nil, fmt.Errorf("reading installed bricks: %w", err)
		}
		for _, brickDir := range brickDirs {
			name, version, ok := strings.Cut(brickDir.Base(), "@")
			if !ok || !validFolderName.MatchString(name) {
				continue
			}
			if _, err := semver.Parse(version); version == "" || err != nil {
				continue
			}
			brick, err := loadInstalledBrick(brickDir, namespace+":"+name)
			if err != nil {
				slog.Warn("skipping invalid installed brick", "path", brickDir, "error", err)
				continue
			}
			res = append(res, InstalledBrick{Namespace: namespace, Version: version, Brick: brick})
		}
	}

	// The versions have already been validated.
	slices.SortFunc(res, func(a, b InstalledBrick) int {
		if c := cmp.Compare(a.Brick.ID, b.Brick.ID); c != 0 {
			return c
		}
		return semver.MustParse(a.Version).CompareTo(semver.MustParse(b.Version))
	})
	return res, nil
}

func loadInstalledBrick(brickDir *paths.Path, id string) (bricksindex.Brick, error) {
	data, err := brickDir.Join(brickConfigFileName).ReadFile()
	if err != nil {
		return bricksindex.Brick{}, err
	}
	var brick bricksindex.Brick
	if err := yaml.Unmarshal(data, &brick); err != nil {
		return bricksindex.Brick{}, fmt.Errorf("invalid %s: %w", brickConfigFileName, err)
	}
	if brick.ID != id {
		return bricksindex.Brick{}, fmt.Errorf("%s declares id %q instead of %q", brickConfigFileName, brick.ID, id)
	}
	brick.FullPath = brickDir
	return brick, nil
}
