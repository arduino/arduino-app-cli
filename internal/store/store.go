// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

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
