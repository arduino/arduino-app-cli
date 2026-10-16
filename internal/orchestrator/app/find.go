// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"errors"

	"github.com/arduino/go-paths-helper"
)

// FindAppsInFolders scans the given paths recursively to find Arduino Apps.
// Paths that do not exist on disk are skipped.
func FindAppsInFolders(pathsToExplore paths.PathList) (paths.PathList, error) {
	var result paths.PathList
	var allErrors error
	for _, p := range pathsToExplore {
		if p.NotExist() {
			continue
		}
		apps, err := p.ReadDirRecursiveFiltered(
			paths.AndFilter( // Recursion filter
				paths.FilterOutNames(".cache"),       // Do not recurse into .cache folders
				paths.NotFilter(DirHasAppDescriptor), // Do not recurse into valid app dirs
			),
			paths.FilterDirectories(),
			paths.FilterOutNames("python", "sketch", ".cache"),
			DirHasAppDescriptor,
		)
		if err != nil {
			allErrors = errors.Join(allErrors, err)
		}
		result.AddAllMissing(apps)
	}
	return result, allErrors
}

// DirHasAppDescriptor returns true if the given directory contains
// an app descriptor file (app.yaml).
func DirHasAppDescriptor(p *paths.Path) bool {
	return p.Join("app.yaml").Exist()
}
