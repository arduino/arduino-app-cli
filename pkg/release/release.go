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
	Libraries []string  `yaml:"libraries,omitempty"`

	Bricks []ReleaseBrick `yaml:"-"`
	Models []ReleaseModel `yaml:"-"`
}

type ReleaseBrick struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name,omitempty"`
	Category string `yaml:"category,omitempty"`
}

type ReleaseModel struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name,omitempty"`
}

// ReadReleaseManifest reads the manifest, the bricks list and the models list of a
// release archive in a single pass
func ReadReleaseManifest(archive *paths.Path) (ReleaseManifest, error) {
	if ext := archive.Ext(); ext != ReleaseArchiveExt {
		return ReleaseManifest{}, fmt.Errorf("%s is not a release archive: expected %q", archive.Base(), ReleaseArchiveExt)
	}

	file, err := archive.Open()
	if err != nil {
		return ReleaseManifest{}, fmt.Errorf("cannot open %s: %w", archive, err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return ReleaseManifest{}, fmt.Errorf("%s is not a release archive: %w", archive.Base(), err)
	}
	defer gzipReader.Close()

	var manifest ReleaseManifest
	// bricksList mirrors prebuild/bricks-list.yaml: a flat list of bricks.
	var bricksList struct {
		Bricks []ReleaseBrick `yaml:"bricks"`
	}
	// modelsList mirrors prebuild/models-list.yaml: each model is a single-entry map keyed by its id.
	var modelsList struct {
		Models []map[string]ReleaseModel `yaml:"models"`
	}
	var foundManifest, foundBricks, foundModels bool

	var releaseName string
	tarReader := tar.NewReader(gzipReader)
	for !foundManifest || !foundBricks || !foundModels {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return ReleaseManifest{}, fmt.Errorf("cannot read %s: %w", archive.Base(), err)
		}

		name := path.Clean(filepath.ToSlash(header.Name))
		root, name, _ := strings.Cut(name, "/")
		if releaseName == "" {
			releaseName = root
		}
		if root != releaseName || root == "" || root == "." || root == ".." {
			return ReleaseManifest{}, fmt.Errorf("%s is not rooted at a single release folder", archive.Base())
		}

		switch name {
		case ReleaseManifestFileName:
			if err := yaml.NewDecoder(tarReader).Decode(&manifest); err != nil {
				return ReleaseManifest{}, fmt.Errorf("cannot read the release manifest: %w", err)
			}
			foundManifest = true
		case BricksListFileName:
			// decode and populate brickRelease
			if err := yaml.NewDecoder(tarReader).Decode(&bricksList); err != nil {
				return ReleaseManifest{}, fmt.Errorf("cannot read %s: %w", BricksListFileName, err)
			}
			foundBricks = true
		case ModelsListFileName:
			// decode and populate modelsListRelease
			if err := yaml.NewDecoder(tarReader).Decode(&modelsList); err != nil {
				return ReleaseManifest{}, fmt.Errorf("cannot read %s: %w", ModelsListFileName, err)
			}
			foundModels = true
		}
	}
	if !foundManifest {
		return ReleaseManifest{}, fmt.Errorf("%s no release manifest, it is not a release", archive.Base())
	}
	if manifest.Name == "" || manifest.Target == "" {
		return ReleaseManifest{}, fmt.Errorf("its release manifest states no name or no target")
	}

	manifest.Bricks = bricksList.Bricks

	manifest.Models = make([]ReleaseModel, 0, len(modelsList.Models))
	for _, entry := range modelsList.Models {
		for id, model := range entry {
			model.ID = id
			manifest.Models = append(manifest.Models, model)
		}
	}

	return manifest, nil
}

func (r ReleaseManifest) GetBricksInfo() []ReleaseBrick {
	return r.Bricks
}

func (r ReleaseManifest) GetModelInfo() []ReleaseModel {
	return r.Models
}
