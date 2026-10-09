// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package release reads a release archive.
package release

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"
)

const (
	ReleaseArchiveExt       = ".ard"
	ReleaseManifestFileName = "release.yaml"
	BricksListFileName      = "prebuild/bricks-list.yaml"
	ModelsListFileName      = "prebuild/models-list.yaml"
)

type AppRelease struct {
	Manifest   ReleaseManifest
	BricksInfo []ReleaseBrick
	ModelsInfo []ReleaseModel
}

// ReleaseManifest is what the archive states of itself: what a board needs to list a
// release and to gate its install. app.Release reads the part that marks an app.
type ReleaseManifest struct {
	Schema       int    `yaml:"schema"`
	Name         string `yaml:"name"`
	ReleaseLabel string `yaml:"release_label,omitempty"`
	Icon         string `yaml:"icon,omitempty"`
	Target       string `yaml:"target"`
	// CreatedAt is when the build ran, UTC.
	CreatedAt time.Time `yaml:"created_at"`
	Notes     string    `yaml:"notes,omitempty"`
	Bricks    []string  `yaml:"bricks,omitempty"`
	Models    []string  `yaml:"models,omitempty"`
	Libraries []string  `yaml:"libraries,omitempty"`
}

type ReleaseBrick struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name,omitempty"`
	Category string `yaml:"category,omitempty"`
}

type ReleaseModel struct {
	ID    string `yaml:"id"`
	Model string `yaml:"model,omitempty"`
}

// ReadReleaseManifest reads the manifest, the bricks list and the models list of a
// release archive in a single pass
func AppReleaseLoad(archive *paths.Path) (AppRelease, error) {
	if ext := archive.Ext(); ext != ReleaseArchiveExt {
		return AppRelease{}, fmt.Errorf("%s is not a release archive: expected %q", archive.Base(), ReleaseArchiveExt)
	}

	file, err := archive.Open()
	if err != nil {
		return AppRelease{}, fmt.Errorf("cannot open %s: %w", archive, err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return AppRelease{}, fmt.Errorf("%s is not a release archive: %w", archive.Base(), err)
	}
	defer gzipReader.Close()

	var bricksList struct {
		Bricks []ReleaseBrick `yaml:"bricks"`
	}
	var modelsList struct {
		Models []map[string]struct {
			Name string `yaml:"name"`
		} `yaml:"models"`
	}
	var manifest ReleaseManifest
	var bricksInfo []ReleaseBrick
	var modelsInfo []ReleaseModel
	var foundManifest, foundBricks, foundModels bool

	var releaseName string
	tarReader := tar.NewReader(gzipReader)
	for !foundManifest || !foundBricks || !foundModels {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return AppRelease{}, fmt.Errorf("cannot read %s: %w", archive.Base(), err)
		}

		name := path.Clean(filepath.ToSlash(header.Name))
		root, name, _ := strings.Cut(name, "/")
		if releaseName == "" {
			releaseName = root
		}
		if root != releaseName || root == "" || root == "." || root == ".." {
			return AppRelease{}, fmt.Errorf("%s is not rooted at a single release folder", archive.Base())
		}

		switch name {
		case ReleaseManifestFileName:
			if err := yaml.NewDecoder(tarReader).Decode(&manifest); err != nil {
				return AppRelease{}, fmt.Errorf("cannot read the release manifest: %w", err)
			}
			foundManifest = true
		case BricksListFileName:
			// decode and populate brickRelease
			if err := yaml.NewDecoder(tarReader).Decode(&bricksList); err != nil {
				return AppRelease{}, fmt.Errorf("cannot read %s: %w", BricksListFileName, err)
			}
			foundBricks = true
		case ModelsListFileName:
			// decode and populate modelsListRelease
			if err := yaml.NewDecoder(tarReader).Decode(&modelsList); err != nil {
				return AppRelease{}, fmt.Errorf("cannot read %s: %w", ModelsListFileName, err)
			}
			foundModels = true
		}
	}
	if !foundManifest {
		return AppRelease{}, fmt.Errorf("%s no release manifest, it is not a release", archive.Base())
	}
	if manifest.Name == "" || manifest.Target == "" {
		return AppRelease{}, fmt.Errorf("its release manifest states no name or no target")
	}

	for _, brick := range bricksList.Bricks {
		if slices.Contains(manifest.Bricks, brick.ID) {
			bricksInfo = append(bricksInfo, brick)
		}
	}
	for _, entry := range modelsList.Models {
		for id, model := range entry {
			if slices.Contains(manifest.Models, id) {
				modelsInfo = append(modelsInfo, ReleaseModel{ID: id, Model: model.Name})
			}
		}
	}

	return AppRelease{
		Manifest:   manifest,
		BricksInfo: bricksInfo,
		ModelsInfo: modelsInfo}, nil
}

func (r AppRelease) GetBricksInfo() []ReleaseBrick {
	return r.BricksInfo
}

func (r AppRelease) GetModelInfo() []ReleaseModel {
	return r.ModelsInfo
}

func (r AppRelease) ReadReleaseManifest() ReleaseManifest {
	return r.Manifest
}
