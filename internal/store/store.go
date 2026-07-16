// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package store

import (
	"path/filepath"

	"github.com/arduino/go-paths-helper"
)

type StaticStore struct {
	baseDir      string
	assetsPath   *paths.Path
	servicesPath string
}

func NewStaticStore(baseDir string) *StaticStore {
	return &StaticStore{
		baseDir:      baseDir,
		assetsPath:   paths.New(baseDir),
		servicesPath: filepath.Join(baseDir, "services")}
}

func (s *StaticStore) GetAssetsFolder() *paths.Path {
	return s.assetsPath
}

func (s *StaticStore) GetServicesFolder() *paths.Path {
	return paths.New(s.servicesPath)
}
