// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

type InstallReleaseResult struct {
	AppID   appid.ID
	Release app.Release
	Name    string
	Path    *paths.Path
}

// InstallRelease unpacks a release archive into the releases dir, as the app it is
// run as: src is the app folder and prebuild is the .cache a start would otherwise
// generate. It is an app like the others, except that it is never written.
func InstallRelease(
	archive *paths.Path,
	idProvider *appid.Provider,
	cfg config.Configuration,
	plat platform.Platform,
) (InstallReleaseResult, error) {
	if archive == nil || archive.NotExist() {
		return InstallReleaseResult{}, fmt.Errorf("%w: %s not found", ErrBadRequest, archive)
	}

	stagingDir, err := mkStagingDir(cfg.ReleasesDir())
	if err != nil {
		return InstallReleaseResult{}, err
	}
	defer removeStagingDir(stagingDir)

	releaseName, manifest, err := extractRelease(archive, stagingDir, appLayout)
	if err != nil {
		return InstallReleaseResult{}, err
	}
	if !app.IsValidFolderName(releaseName) {
		return InstallReleaseResult{}, fmt.Errorf("%w: release name %q is not valid", ErrBadRequest, releaseName)
	}

	if manifest.Version == "" || manifest.Target == "" {
		return InstallReleaseResult{}, fmt.Errorf("%w: %s carries no release manifest, it is not a release", ErrBadRequest, archive.Base())
	}
	// A newer layout may hold the facts elsewhere, so the ones read here are not trusted.
	if manifest.Schema > ReleaseManifestSchema {
		return InstallReleaseResult{}, fmt.Errorf("%w: %s is schema %d and needs a newer cli", ErrBadRequest, archive.Base(), manifest.Schema)
	}
	if manifest.Target != plat.BoardName {
		return InstallReleaseResult{}, fmt.Errorf("%w: the release is built for %s, this board is a %s", ErrBadRequest, manifest.Target, plat.BoardName)
	}
	release := app.Release{ID: releaseName, Version: manifest.Version, Target: manifest.Target}

	// The app folder as it runs: the release ships neither the data nor its content.
	if err := stagingDir.Join("data").MkdirAll(); err != nil {
		return InstallReleaseResult{}, fmt.Errorf("failed to create the data dir: %w", err)
	}
	if err := writeReleaseMarker(stagingDir, release); err != nil {
		return InstallReleaseResult{}, err
	}
	if _, err := app.Load(stagingDir); err != nil {
		return InstallReleaseResult{}, fmt.Errorf("%w: the release does not hold a valid app: %v", ErrBadRequest, err)
	}

	releasePath := cfg.ReleasesDir().Join(releaseName)
	if releasePath.Exist() {
		return InstallReleaseResult{}, fmt.Errorf("%w: %s is already installed", ErrAppAlreadyExists, releaseName)
	}
	if err := stagingDir.Rename(releasePath); err != nil {
		return InstallReleaseResult{}, fmt.Errorf("failed to install the release: %w", err)
	}

	appID, err := idProvider.IDFromPath(releasePath)
	if err != nil {
		return InstallReleaseResult{}, err
	}
	return InstallReleaseResult{
		AppID:   appID,
		Release: release,
		Name:    strings.TrimSuffix(releaseName, "-"+release.Version+"-"+release.Target),
		Path:    releasePath,
	}, nil
}

// appLayout is the app the release is run as: src is the app folder and prebuild is
// the .cache a start would otherwise generate. The rest is the release alone.
func appLayout(entry string) string {
	switch dir, rest, _ := strings.Cut(entry, "/"); dir {
	case releaseSrcDir:
		return rest
	case app.PrebuildDirName:
		return path.Join(".cache", rest)
	default:
		return ""
	}
}

// extractRelease unpacks the archive into destDir, each entry where layout puts it.
// It returns the release name, the folder the archive is rooted at, and the manifest.
func extractRelease(archive *paths.Path, destDir *paths.Path, layout func(entry string) string) (string, ReleaseManifest, error) {
	file, err := archive.Open()
	if err != nil {
		return "", ReleaseManifest{}, fmt.Errorf("cannot open %s: %w", archive, err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return "", ReleaseManifest{}, fmt.Errorf("%w: %s is not a release archive: %v", ErrBadRequest, archive.Base(), err)
	}
	defer gzipReader.Close()

	// The files are written through a root, so that neither a name nor a symlink of
	// the archive can write outside of the folder being installed.
	destRoot, err := os.OpenRoot(destDir.String())
	if err != nil {
		return "", ReleaseManifest{}, fmt.Errorf("failed to open the staging dir: %w", err)
	}
	defer destRoot.Close()

	var releaseName string
	var manifest ReleaseManifest
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", ReleaseManifest{}, fmt.Errorf("%w: cannot read %s: %v", ErrBadRequest, archive.Base(), err)
		}

		name := path.Clean(filepath.ToSlash(header.Name))
		root, entry, _ := strings.Cut(name, "/")
		if releaseName == "" {
			releaseName = root
		}
		if root != releaseName || root == "" || root == "." || root == ".." {
			return "", ReleaseManifest{}, fmt.Errorf("%w: %s is not rooted at a single release folder", ErrBadRequest, archive.Base())
		}
		if entry == "" {
			continue
		}
		// The manifest is archived first, so the facts are known before anything is written.
		if entry == ReleaseManifestFileName {
			content, err := io.ReadAll(tarReader)
			if err == nil {
				err = yaml.Unmarshal(content, &manifest)
			}
			if err != nil {
				return "", ReleaseManifest{}, fmt.Errorf("%w: cannot read the manifest of %s: %v", ErrBadRequest, archive.Base(), err)
			}
			continue
		}

		target := layout(entry)
		if target == "" {
			continue
		}

		mode := header.FileInfo().Mode()
		switch header.Typeflag {
		case tar.TypeDir:
			err = destRoot.MkdirAll(target, mode.Perm())
		case tar.TypeSymlink:
			// Kept as it is: the venv links to what the runner image has, at the
			// absolute path it has it.
			if err = destRoot.MkdirAll(path.Dir(target), 0755); err == nil {
				err = destRoot.Symlink(header.Linkname, target)
			}
		case tar.TypeReg:
			if err = destRoot.MkdirAll(path.Dir(target), 0755); err == nil {
				err = writeReleaseFile(destRoot, target, mode.Perm(), tarReader)
			}
		default:
			slog.Warn("skipping the release entry", slog.String("name", name), slog.String("type", string(header.Typeflag)))
		}
		if err != nil {
			return "", ReleaseManifest{}, fmt.Errorf("failed to extract %s: %w", name, err)
		}
	}

	if releaseName == "" {
		return "", ReleaseManifest{}, fmt.Errorf("%w: %s is empty", ErrBadRequest, archive.Base())
	}
	return releaseName, manifest, nil
}

func writeReleaseFile(destRoot *os.Root, target string, perm os.FileMode, content io.Reader) error {
	file, err := destRoot.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, content); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func writeReleaseMarker(appDir *paths.Path, release app.Release) error {
	marker, err := yaml.Marshal(release)
	if err != nil {
		return err
	}
	if err := appDir.Join(app.ReleaseFileName).WriteFile(marker); err != nil {
		return fmt.Errorf("failed to write the release marker: %w", err)
	}
	return nil
}

// mkStagingDir stages an install beside what it installs, so that installing it is a
// rename on the same filesystem and nothing half extracted is ever started.
func mkStagingDir(installDir *paths.Path) (*paths.Path, error) {
	if err := installDir.MkdirAll(); err != nil {
		return nil, fmt.Errorf("failed to create %s: %w", installDir, err)
	}
	stagingDir, err := paths.MkTempDir(installDir.String(), "tmp_install_")
	if err != nil {
		return nil, fmt.Errorf("failed to create the staging dir: %w", err)
	}
	return stagingDir, nil
}

func removeStagingDir(stagingDir *paths.Path) {
	if err := stagingDir.RemoveAll(); err != nil && !os.IsNotExist(err) {
		slog.Warn("cannot remove the install staging dir", slog.String("path", stagingDir.String()), slog.String("error", err.Error()))
	}
}
