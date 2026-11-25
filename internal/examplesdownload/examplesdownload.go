// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package examplesdownload downloads the examples of the arduino/app-bricks-examples
// release, the same ones the `build-deb:clone-examples` task clones with git. This
// fetches the release tarball straight from GitHub, so it needs no external tool besides
// a network connection.
package examplesdownload

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/arduino/go-paths-helper"
	"go.bug.st/downloader/v3"
)

const (
	owner = "arduino"
	repo  = "app-bricks-examples"

	// maxExtractedFileSize for examples
	maxExtractedFileSize = 10 << 20 // 10MiB
)

// included are the entries of the repository root that make up the examples.
var included = []string{"core-and-foundational", "inspirational", "bricks", "examples.json"}

// Downloader fetches the examples of a version from GitHub.
type Downloader struct{}

// NewDownloader returns a ready-to-use Downloader.
func NewDownloader() *Downloader {
	return &Downloader{}
}

// Download fetches the tarball of the given examples version and extracts the examples
// into dest/examples, wiping that directory first.
func (d *Downloader) Download(ctx context.Context, version string, dest *paths.Path) error {
	url := fmt.Sprintf("https://github.com/%s/%s/archive/refs/tags/%s.tar.gz", owner, repo, version)

	// Downloaded to a temp file, not buffered in memory: go.bug.st/downloader streams it
	// to disk and extract reads it back from there.
	tarball, err := os.CreateTemp("", "examples-*.tar.gz")
	if err != nil {
		return fmt.Errorf("cannot create a temporary file: %w", err)
	}
	tarballPath := tarball.Name()
	_ = tarball.Close()
	defer os.Remove(tarballPath)

	config := downloader.GetDefaultConfig()
	config.InactivityTimeout = 30 * time.Second
	if err := downloader.DownloadWithConfig(ctx, tarballPath, url, config); err != nil {
		return fmt.Errorf("cannot download examples %s: %w", version, err)
	}

	examplesDir := dest.Join("examples")
	if err := examplesDir.RemoveAll(); err != nil {
		return fmt.Errorf("cannot clean up %s: %w", examplesDir, err)
	}
	if err := extract(tarballPath, examplesDir); err != nil {
		_ = examplesDir.RemoveAll()
		return fmt.Errorf("cannot extract examples %s: %w", version, err)
	}
	return nil
}

// extract unpacks the examples of the tarball into dest. GitHub wraps the repository in
// a single <repo>-<version>/ folder, which is stripped.
func extract(tarballPath string, dest *paths.Path) error {
	f, err := os.Open(tarballPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("not a valid gzip: %w", err)
	}
	defer gz.Close()

	if err := dest.MkdirAll(); err != nil {
		return err
	}

	var extracted int
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		_, relPath, ok := strings.Cut(header.Name, "/")
		if !ok || !isIncluded(relPath) {
			continue
		}

		// Names come from the network: the check is on the cleaned path, with `..` or an
		// absolute path being refused rather than escaping dest.
		target := dest.JoinPath(paths.New(relPath))
		if rel, err := target.RelFrom(dest); err != nil || strings.HasPrefix(rel.String(), "..") {
			return fmt.Errorf("invalid path %q in the tarball", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := target.MkdirAll(); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := target.Parent().MkdirAll(); err != nil {
				return err
			}
			if err := copyFile(tr, header, target); err != nil {
				return err
			}
			extracted++
		}
	}

	if extracted == 0 {
		return fmt.Errorf("the tarball has no examples")
	}
	return nil
}

func isIncluded(relPath string) bool {
	for _, name := range included {
		if relPath == name || strings.HasPrefix(relPath, name+"/") {
			return true
		}
	}
	return false
}

func copyFile(src io.Reader, header *tar.Header, target *paths.Path) error {
	out, err := os.OpenFile(target.String(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode)&0o777)
	if err != nil {
		return err
	}
	defer out.Close()

	// The +1 lets written exceed maxExtractedFileSize by one, so going over it is
	// detected instead of silently truncating the file at the limit.
	written, err := io.Copy(out, io.LimitReader(src, maxExtractedFileSize+1))
	if err != nil {
		return err
	}
	if written > maxExtractedFileSize {
		return fmt.Errorf("%s is larger than the %d bytes limit", header.Name, maxExtractedFileSize)
	}
	return out.Close()
}
