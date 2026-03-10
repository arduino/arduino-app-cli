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

package store

import (
	"path/filepath"

	"github.com/arduino/go-paths-helper"
)

type StaticStore struct {
	baseDir     string
	composePath string
	assetsPath  *paths.Path
}

func NewStaticStore(baseDir string) *StaticStore {
	return &StaticStore{
		baseDir:     baseDir,
		composePath: filepath.Join(baseDir, "compose"),
		assetsPath:  paths.New(baseDir),
	}
}

func (s *StaticStore) GetAssetsFolder() *paths.Path {
	return s.assetsPath
}

func (s *StaticStore) GetComposeFolder() *paths.Path {
	return paths.New(s.composePath)
}
