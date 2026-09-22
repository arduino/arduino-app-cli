// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package release reads the facts a release archive states of itself, without
// extracting or installing it. It is meant for callers, such as App Lab, that need
// those facts before an install runs, so that no daemon round trip is needed for a
// file the caller already has on disk.
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

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

// ReleaseInfo is what a release archive states of itself, read straight off it.
type ReleaseInfo struct {
	Schema int    `yaml:"schema"`
	Name   string `yaml:"name"`
	Target string `yaml:"target"`
	// CreatedAt is when the build ran, UTC.
	CreatedAt time.Time `yaml:"created_at"`
	// Notes is the release note value, if present.
	Notes     string                      `yaml:"notes,omitempty"`
	Bricks    []orchestrator.ReleaseBrick `yaml:"bricks,omitempty"`
	Models    []orchestrator.ReleaseModel `yaml:"models,omitempty"`
	Libraries []string                    `yaml:"libraries,omitempty"`
}

// ReadReleaseInfo reads the release file extracting and
// stops once the release.yaml file is found
func ReadReleaseInfo(archive *paths.Path) (ReleaseInfo, error) {
	if ext := archive.Ext(); ext != orchestrator.ReleaseArchiveExt {
		return ReleaseInfo{}, fmt.Errorf("%s is not a release archive: expected %q", archive.Base(), orchestrator.ReleaseArchiveExt)
	}

	file, err := archive.Open()
	if err != nil {
		return ReleaseInfo{}, fmt.Errorf("cannot open %s: %w", archive, err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return ReleaseInfo{}, fmt.Errorf("%s is not a release archive: %w", archive.Base(), err)
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
			return ReleaseInfo{}, fmt.Errorf("cannot read %s: %w", archive.Base(), err)
		}

		name := path.Clean(filepath.ToSlash(header.Name))
		root, entry, _ := strings.Cut(name, "/")
		if releaseName == "" {
			releaseName = root
		}
		if root != releaseName || root == "" || root == "." || root == ".." {
			return ReleaseInfo{}, fmt.Errorf("%s is not rooted at a single release folder", archive.Base())
		}
		if entry != app.ReleaseManifestFileName {
			continue
		}

		var info ReleaseInfo
		if err := yaml.NewDecoder(tarReader).Decode(&info); err != nil {
			return ReleaseInfo{}, fmt.Errorf("cannot read the release manifest: %w", err)
		}
		if info.Name == "" || info.Target == "" {
			return ReleaseInfo{}, fmt.Errorf("its release manifest states no name or no target")
		}
		return info, nil
	}

	return ReleaseInfo{}, fmt.Errorf("%s no release manifest, it is not a release", archive.Base())
}
