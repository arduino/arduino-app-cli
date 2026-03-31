// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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
