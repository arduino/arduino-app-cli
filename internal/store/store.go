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

package store

import (
	"fmt"
	"path/filepath"

	"github.com/arduino/go-paths-helper"
)

type StaticStore struct {
	baseDir      string
	composePath  string
	assetsPath   *paths.Path
	servicesPath string
}

func NewStaticStore(baseDir string) *StaticStore {
	return &StaticStore{
		baseDir:      baseDir,
		composePath:  filepath.Join(baseDir, "compose"),
		assetsPath:   paths.New(baseDir),
		servicesPath: filepath.Join(baseDir, "services")}
}

func (s *StaticStore) SaveComposeFolderTo(dst string) error {
	composeFS := s.GetComposeFolder()
	dstPath := paths.New(dst)
	_ = dstPath.RemoveAll()
	if err := composeFS.CopyDirTo(dstPath); err != nil {
		return fmt.Errorf("failed to copy assets directory: %w", err)
	}
	return nil
}

func (s *StaticStore) GetAssetsFolder() *paths.Path {
	return s.assetsPath
}

func (s *StaticStore) GetComposeFolder() *paths.Path {
	return paths.New(s.composePath)
}

func (s *StaticStore) GetServicesFolder() *paths.Path {
	return paths.New(s.servicesPath)
}
