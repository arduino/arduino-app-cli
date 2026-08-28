// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/arduino/go-paths-helper"
	"github.com/docker/cli/cli/command"
	"github.com/gosimple/slug"

	"github.com/arduino/arduino-app-cli/internal/dockerhelper"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

// A release is an app frozen with all its dependencies: <name>-<version>-<target>/
// holds NOTES.md, src/ as authored and prebuild/, which becomes .cache/ on install.

type BuildReleaseRequest struct {
	// Target defaults to the board running the build.
	Target string
	// Version defaults to a UTC timestamp, which keeps the releases of an app ordered.
	Version string
	// Notes ships as NOTES.md.
	Notes *paths.Path
	// Output is the archive, or the directory to write it in. Defaults to the cwd.
	Output    *paths.Path
	Overwrite bool
}

type BuildReleaseResult struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Target  string `json:"target"`
	Archive string `json:"archive"`
}

// The release facts, stamped on the main service of the frozen compose: there is no
// manifest, the rest is in app.yaml and in the compose file.
const (
	ReleaseVersionLabel = "cc.arduino.release.version"
	ReleaseTargetLabel  = "cc.arduino.release.target"
)

// BuildRelease provisions an app for the target board and builds its python
// environment into a release archive, without touching the app folder.
func BuildRelease(
	ctx context.Context,
	docker command.Cli,
	provisioner *Provision,
	modelsIndex *modelsindex.ModelsIndex,
	bricksIndex *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
	appToBuild app.ArduinoApp,
	req BuildReleaseRequest,
	cfg config.Configuration,
	plat platform.Platform,
	cb func(StreamMessage),
) (BuildReleaseResult, error) {
	if cb == nil {
		cb = func(StreamMessage) {}
	}

	bricksIndex = bricksIndex.WithAppBricks(appToBuild.LocalBricks)
	if err := checkBricks(ctx, appToBuild.Descriptor.Bricks, bricksIndex, modelsIndex); err != nil {
		return BuildReleaseResult{}, err
	}

	if err := checkPortCollisions(appToBuild.Descriptor, bricksIndex, servicesIndex); err != nil {
		return BuildReleaseResult{}, err
	}

	// TODO: fail when a service a brick requires is missing for the target. The
	// generator skips it, so it would be missing from the archive for good.

	version := req.Version
	if version == "" {
		version = time.Now().UTC().Format("20060102-150405")
	}
	name := slug.Make(appToBuild.Name)
	if name == "" {
		return BuildReleaseResult{}, fmt.Errorf("%w: the app has no name", ErrBadRequest)
	}
	releaseName := fmt.Sprintf("%s-%s-%s", name, version, plat.BoardName)

	archivePath, err := releaseArchivePath(releaseName, req)
	if err != nil {
		return BuildReleaseResult{}, err
	}

	// Staged outside of the app: a release must not inherit its .cache.
	stagingDir, err := cfg.MkTempBuildDir()
	if err != nil {
		return BuildReleaseResult{}, fmt.Errorf("failed to create the staging dir: %w", err)
	}
	defer func() {
		if err := stagingDir.RemoveAll(); err != nil {
			slog.Warn("cannot remove the release staging dir", slog.String("path", stagingDir.String()), slog.String("error", err.Error()))
		}
	}()

	releaseDir := stagingDir.Join(releaseName)
	srcDir := releaseDir.Join("src")
	prebuildDir := releaseDir.Join("prebuild")

	cb(StreamMessage{progress: &Progress{Name: "copying the app", Progress: 0.0}})
	if err := stageReleaseSrc(appToBuild, srcDir); err != nil {
		return BuildReleaseResult{}, err
	}

	if req.Notes != nil {
		if err := req.Notes.CopyTo(releaseDir.Join("NOTES.md")); err != nil {
			return BuildReleaseResult{}, fmt.Errorf("failed to copy the release notes: %w", err)
		}
	}

	// Loaded back from the staging dir: the provisioning must read the copy that ships.
	stagedApp, err := app.Load(srcDir)
	if err != nil {
		return BuildReleaseResult{}, fmt.Errorf("the staged app is not valid: %w", err)
	}

	// Before the environment, as in a start: it is quick and it is where a build
	// fails, so it does not make anyone wait for the venv to find it out.
	cb(StreamMessage{data: "freezing the compose files", progress: &Progress{Name: "compose files", Progress: 10.0}})
	appEnv := appEnvironment(ctx, stagedApp, bricksIndex, modelsIndex, plat)
	buildOpts := BuildOptions{
		ProjectName: slug.Make(releaseName),
		ComposesDir: prebuildDir.Join("composes"),
		Labels: map[string]string{
			ReleaseVersionLabel: version,
			ReleaseTargetLabel:  plat.BoardName,
		},
	}
	if err := provisioner.Resolve(&stagedApp, prebuildDir, bricksIndex, servicesIndex, cfg, appEnv, plat, buildOpts); err != nil {
		return BuildReleaseResult{}, fmt.Errorf("failed to freeze the compose files: %w", err)
	}

	cb(StreamMessage{data: "building the python environment", progress: &Progress{Name: "python environment", Progress: 20.0}})
	if err := buildPythonEnv(ctx, docker, srcDir, prebuildDir, cb); err != nil {
		return BuildReleaseResult{}, err
	}

	cb(StreamMessage{data: "writing " + archivePath.Base(), progress: &Progress{Name: "archive", Progress: 90.0}})
	if err := writeReleaseArchive(releaseDir, archivePath); err != nil {
		return BuildReleaseResult{}, err
	}

	cb(StreamMessage{progress: &Progress{Name: "", Progress: 100.0}})
	return BuildReleaseResult{
		Name:    name,
		Version: version,
		Target:  plat.BoardName,
		Archive: archivePath.String(),
	}, nil
}

// A gzipped tar and not a zip: the venv needs symlinks and exec bits preserved.
const ReleaseArchiveExt = ".arduinoapp"

// releaseArchivePath resolves where the archive goes, without creating it.
func releaseArchivePath(releaseName string, req BuildReleaseRequest) (*paths.Path, error) {
	fileName := releaseName + ReleaseArchiveExt

	archivePath := paths.New(fileName)
	if req.Output != nil {
		archivePath = req.Output
		if archivePath.IsDir() {
			archivePath = archivePath.Join(fileName)
		}
	}
	archivePath, err := archivePath.Abs()
	if err != nil {
		return nil, err
	}

	if archivePath.Exist() && !req.Overwrite {
		return nil, fmt.Errorf("%w: %s already exists", ErrBadRequest, archivePath)
	}
	return archivePath, nil
}

// stageReleaseSrc copies the app folder as authored: .cache is resolved anew by the
// build and data is created empty on install.
func stageReleaseSrc(appToBuild app.ArduinoApp, srcDir *paths.Path) error {
	if err := srcDir.MkdirAll(); err != nil {
		return fmt.Errorf("failed to create the release src dir: %w", err)
	}

	skipFilter := appSourceFilter(false)
	entries, err := appToBuild.FullPath.ReadDirRecursiveFiltered(skipFilter, skipFilter)
	if err != nil {
		return fmt.Errorf("failed to read the app folder: %w", err)
	}
	for _, entry := range entries {
		relPath, err := entry.RelFrom(appToBuild.FullPath)
		if err != nil {
			return err
		}
		dst := srcDir.JoinPath(relPath)

		// Stat and CopyTo follow the link, as the export does: a release carries what
		// the link points at, which may well be outside the app folder.
		info, err := entry.Stat()
		if err != nil {
			if errors.Is(err, syscall.ELOOP) {
				continue
			}
			return fmt.Errorf("failed to read %s: %w", relPath, err)
		}

		if info.IsDir() {
			if err := dst.MkdirAll(); err != nil {
				return fmt.Errorf("failed to create %s: %w", relPath, err)
			}
			continue
		}
		if err := dst.Parent().MkdirAll(); err != nil {
			return fmt.Errorf("failed to create %s: %w", relPath.Parent(), err)
		}
		if err := entry.CopyTo(dst); err != nil {
			return fmt.Errorf("failed to copy %s: %w", relPath, err)
		}
	}

	return nil
}

// buildPythonEnv builds the python environment in the runner image, as run.sh would do
// at the first start, and leaves it in the prebuild dir.
func buildPythonEnv(ctx context.Context, docker command.Cli, srcDir *paths.Path, prebuildDir *paths.Path, cb func(StreamMessage)) error {
	if err := prebuildDir.MkdirAll(); err != nil {
		return fmt.Errorf("failed to create the prebuild dir: %w", err)
	}

	// TODO: dev image, to be replaced by cfg.PythonImage once prepare is released.
	const prepareImage = "ghcr.io/lucarin91/app-bricks/python-apps-base:dev-add-prepare-command"

	output := NewCallbackWriter(func(line string) {
		cb(StreamMessage{data: line})
	})
	err := dockerhelper.Run(ctx, docker.Client(), dockerhelper.RunOptions{
		Image: prepareImage,
		Cmd:   []string{"prepare"},
		Binds: []string{
			srcDir.String() + ":/app",
			prebuildDir.String() + ":/app/.cache",
		},
		Stdout: output,
		Stderr: output,
	})
	if err != nil {
		return fmt.Errorf("failed to build the python environment: %w", err)
	}
	return nil
}

// writeReleaseArchive writes releaseDir as a gzipped tar rooted at its own name.
// Symlinks and modes are kept: the venv relies on both.
func writeReleaseArchive(releaseDir *paths.Path, archivePath *paths.Path) error {
	file, err := archivePath.Create()
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", archivePath, err)
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	stagingDir := releaseDir.Parent()
	// The files are read through the staging dir, so that a symlink cannot make the
	// archive pick up something from outside of it.
	stagingRoot, err := os.OpenRoot(stagingDir.String())
	if err != nil {
		return fmt.Errorf("failed to open the staging dir: %w", err)
	}
	defer stagingRoot.Close()

	err = func() error {
		// The same rule the staging used, except that the venv is shipped whole: a
		// package of it may well have a data folder.
		keep := appSourceFilter(true)
		// A symlink is archived as it is, so it is never walked into.
		notSymlink := func(p *paths.Path) bool {
			info, err := p.Lstat()
			return err == nil && info.Mode()&os.ModeSymlink == 0
		}

		entries, err := releaseDir.ReadDirRecursiveFiltered(paths.AndFilter(keep, notSymlink), keep)
		if err != nil {
			return err
		}

		// The archive is rooted at the release folder, so that is the first entry.
		for _, entry := range append(paths.PathList{releaseDir}, entries...) {
			info, err := entry.Lstat()
			if err != nil {
				return err
			}
			relPath, err := entry.RelFrom(stagingDir)
			if err != nil {
				return err
			}

			var linkTarget string
			if info.Mode()&os.ModeSymlink != 0 {
				if linkTarget, err = os.Readlink(entry.String()); err != nil {
					return err
				}
			}

			header, err := tar.FileInfoHeader(info, linkTarget)
			if err != nil {
				return err
			}
			// A tar name is always slash separated.
			header.Name = filepath.ToSlash(relPath.String())
			// The install decides who owns the files.
			header.Uid, header.Gid = 0, 0
			header.Uname, header.Gname = "", ""

			if err := tarWriter.WriteHeader(header); err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				continue
			}

			content, err := stagingRoot.Open(relPath.String())
			if err != nil {
				return err
			}
			_, err = io.Copy(tarWriter, content)
			content.Close()
			if err != nil {
				return err
			}
		}
		return nil
	}()
	if err != nil {
		// Do not leave a half written archive behind.
		_ = archivePath.Remove()
		return fmt.Errorf("failed to write %s: %w", archivePath, err)
	}
	return nil
}
