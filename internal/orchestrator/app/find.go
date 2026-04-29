// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"strings"

	"github.com/arduino/go-paths-helper"
)

// FindAppsInFolder scans the given paths recursively to find Arduino Apps and
// returns the list of found app paths.
func FindAppsInFolder(pathToExplore *paths.Path) (paths.PathList, error) {
	return pathToExplore.ReadDirRecursiveFiltered(
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
