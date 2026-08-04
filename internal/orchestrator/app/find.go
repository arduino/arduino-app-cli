// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"errors"
	"strings"

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
				paths.NotFilter(IsTmpAppDir),         // Do not recurse into temporary apps
				paths.NotFilter(DirHasAppDescriptor), // Do not recurse into valid app dirs
			),
			paths.FilterDirectories(),
			paths.FilterOutNames("python", "sketch", ".cache"),
			paths.NotFilter(IsTmpAppDir),
			DirHasAppDescriptor,
		)
		if err != nil {
			allErrors = errors.Join(allErrors, err)
		}
		result.AddAllMissing(apps)
	}
	return result, allErrors
}

const tmpAppPrefix = ".tmp_"

// DirHasAppDescriptor returns true if the given directory contains
// an app descriptor file (app.yaml).
func DirHasAppDescriptor(p *paths.Path) bool {
	return p.Join("app.yaml").Exist()
}

// IsTmpAppDir returns true if the app path is a temporary app
// that should not be listed (neither in the broken apps).
func IsTmpAppDir(p *paths.Path) bool {
	return strings.HasPrefix(p.Base(), tmpAppPrefix)
}

// MkTmpAppDir creates a temporary app directory inside the given
// parent directory.
func MkTmpAppDir(parentDir *paths.Path) (*paths.Path, error) {
	return paths.MkTempDir(parentDir.String(), tmpAppPrefix)
}
