// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

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
		// TODO: DirHasAppDescriptor ?
	)
}

const tmpAppPrefix = ".tmp_"

// DirHasAppDescriptor returns true if the given directory contains
// an app descriptor file (app.yaml or app.yml).
func DirHasAppDescriptor(p *paths.Path) bool {
	return p.Join("app.yaml").Exist() || p.Join("app.yml").Exist()
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
