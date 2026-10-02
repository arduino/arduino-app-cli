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
)

// ReleaseManifest is what the archive states of itself: what a board needs to list a
// release and to gate its install. app.Release reads the part that marks an app.
type ReleaseManifest struct {
	Schema       int    `yaml:"schema"`
	Name         string `yaml:"name"`
	ReleaseLabel string `yaml:"release_label,omitempty"`
	Target       string `yaml:"target"`
	// CreatedAt is when the build ran, UTC.
	CreatedAt time.Time      `yaml:"created_at"`
	Notes     string         `yaml:"notes,omitempty"`
	Bricks    []ReleaseBrick `yaml:"bricks,omitempty"`
	Models    []ReleaseModel `yaml:"models,omitempty"`
	Libraries []string       `yaml:"libraries,omitempty"`
}

// ReleaseBrick is a brick of the app as the build wired it: the model is part of what a
// release freezes, so it is stated here and not derived again on the board.
type ReleaseBrick struct {
	ID    string `yaml:"id"`
	Model string `yaml:"model,omitempty"`
}

// ReleaseModel is an AI model the app is built with. The id holds the runner and the
// variant, which is as close to a version as a model gets.
type ReleaseModel struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name,omitempty"`
}

// ReadReleaseManifest reads the release file extracting and
// stops once the release.yaml file is found
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

	var releaseName string
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return ReleaseManifest{}, fmt.Errorf("cannot read %s: %w", archive.Base(), err)
		}

		name := path.Clean(filepath.ToSlash(header.Name))
		root, entry, _ := strings.Cut(name, "/")
		if releaseName == "" {
			releaseName = root
		}
		if root != releaseName || root == "" || root == "." || root == ".." {
			return ReleaseManifest{}, fmt.Errorf("%s is not rooted at a single release folder", archive.Base())
		}
		if entry != ReleaseManifestFileName {
			continue
		}

		var info ReleaseManifest
		if err := yaml.NewDecoder(tarReader).Decode(&info); err != nil {
			return ReleaseManifest{}, fmt.Errorf("cannot read the release manifest: %w", err)
		}
		if info.Name == "" || info.Target == "" {
			return ReleaseManifest{}, fmt.Errorf("its release manifest states no name or no target")
		}
		return info, nil
	}

	return ReleaseManifest{}, fmt.Errorf("%s no release manifest, it is not a release", archive.Base())
}
