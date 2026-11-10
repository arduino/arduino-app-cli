// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

// BuiltInNamespace is the namespace of the bricks shipped with the cli. For now they are
// looked up only in the built-in index.
const BuiltInNamespace = "arduino"

// ErrNotInstalled is returned when the version a brick resolves to is not installed.
var ErrNotInstalled = errors.New("brick not installed")

// Reference is a brick used by an app: the version is empty when it is not pinned.
type Reference struct {
	ID      string
	Version string
}

type ResolvedBrick struct {
	ID      string
	Version string
	// Path is where the version is installed, or will be.
	Path      *paths.Path
	Installed bool
	// IndexBrick and Release are the entries of the saved index. They are nil when the
	// version is installed and there is no saved index.
	IndexBrick *Brick
	Release    *Release
}

// IsThirdParty reports whether the brick is not built-in.
func IsThirdParty(id string) bool {
	namespace, _, err := bricksindex.ParseBrickID(id)
	return err == nil && namespace != "" && namespace != BuiltInNamespace
}

// Resolve returns the version each third-party reference resolves to, without accessing the network:
//   - a pinned reference resolves to its version;
//   - an unpinned reference resolves to the latest installed version or, if none is
//     installed, to the latest version of the saved index.
func Resolve(index *Index, bricksDir *paths.Path, refs []Reference) ([]ResolvedBrick, error) {
	var res []ResolvedBrick
	for _, ref := range refs {
		if !IsThirdParty(ref.ID) {
			if ref.Version != "" {
				return nil, fmt.Errorf("brick %q cannot be pinned to a version: only third-party bricks can", ref.ID)
			}
			continue
		}
		resolved, err := resolve(index, bricksDir, ref)
		if err != nil {
			return nil, err
		}
		res = append(res, resolved)
	}
	return res, nil
}

// LoadResolved loads the definitions of the resolved bricks for the platform. Every brick
// must be installed and support the board's platform.
func LoadResolved(plat platform.Platform, resolved []ResolvedBrick) ([]bricksindex.Brick, error) {
	bricks := make([]bricksindex.Brick, 0, len(resolved))
	for _, r := range resolved {
		if !r.Installed {
			return nil, fmt.Errorf("%w: %s@%s, install it with 'arduino-app-cli brick install %s@%s'", ErrNotInstalled, r.ID, r.Version, r.ID, r.Version)
		}
		brick, err := loadInstalledBrick(plat, r.Path, r.ID)
		if err != nil {
			return nil, fmt.Errorf("loading brick %s@%s: %w", r.ID, r.Version, err)
		}
		if !brick.SupportsBoard(plat) {
			return nil, fmt.Errorf("brick %s@%s does not support the board %s", r.ID, r.Version, plat.BoardName)
		}
		bricks = append(bricks, brick)
	}
	return bricks, nil
}

// Source is where the third-party bricks are loaded from: the saved index, the folder
// they are installed in and the platform they are loaded for.
type Source struct {
	IndexDir  *paths.Path
	IndexURL  *url.URL
	BricksDir *paths.Path
	Platform  platform.Platform
	// RequireIndex makes Load fail without a saved index
	RequireIndex bool
}

// NewSource returns the source of the third-party bricks of the board.
func NewSource(cfg config.Configuration, plat platform.Platform) Source {
	return Source{IndexDir: cfg.BricksIndexDir(), IndexURL: cfg.BricksIndexURL, BricksDir: cfg.BricksDir(), Platform: plat}
}

// NewStrictSource is NewSource with RequireIndex set.
func NewStrictSource(cfg config.Configuration, plat platform.Platform) Source {
	src := NewSource(cfg, plat)
	src.RequireIndex = true
	return src
}

// Install installs the third-party bricks the references resolve to that are not installed
// yet, calling progress before each one. The index is updated when there is none on the
// board, even if every brick is installed, and when it does not list a brick or its version.
func (s Source) Install(ctx context.Context, client *http.Client, userAgent string, refs []Reference, progress func(string)) error {
	if !slices.ContainsFunc(refs, func(r Reference) bool { return IsThirdParty(r.ID) }) {
		return nil
	}
	update := func() (*Index, error) {
		progress("updating the bricks index")
		res, err := Update(ctx, client, s.IndexURL, s.IndexDir, userAgent)
		return res.Index, err
	}

	index, err := Load(s.IndexDir)
	updated := err != nil
	if updated {
		// A broken index is replaced, like a missing one.
		slog.Debug("cannot load the bricks index", "error", err)
		if index, err = update(); err != nil {
			return err
		}
	}
	resolved, err := Resolve(index, s.BricksDir, refs)
	if !updated && (errors.Is(err, ErrBrickNotFound) || errors.Is(err, ErrReleaseNotFound)) {
		updatedIndex, updateErr := update()
		if updateErr != nil {
			return errors.Join(err, updateErr)
		}
		resolved, err = Resolve(updatedIndex, s.BricksDir, refs)
	}
	if err != nil {
		return err
	}

	for _, r := range resolved {
		if r.Installed {
			continue
		}
		progress(fmt.Sprintf("installing the brick %s@%s", r.ID, r.Version))
		if _, err := Install(ctx, client, s.BricksDir, r.IndexBrick, r.Release, userAgent); err != nil {
			return err
		}
	}
	return nil
}

// Load resolves the references against the saved index and loads the installed bricks
// they resolve to, skipping the references that are not third-party. Each brick is
// resolved on its own, so one that fails does not hide the others: on error, the bricks
// that could be loaded are still returned.
func (s Source) Load(refs []Reference) ([]bricksindex.Brick, error) {
	if !slices.ContainsFunc(refs, func(r Reference) bool { return IsThirdParty(r.ID) || r.Version != "" }) {
		return nil, nil
	}
	index, err := Load(s.IndexDir)
	if err != nil && (s.RequireIndex || !errors.Is(err, ErrIndexNotFound)) {
		return nil, err
	}

	var bricks []bricksindex.Brick
	var errs []error
	for _, ref := range refs {
		loaded, err := loadReference(index, s.BricksDir, s.Platform, ref)
		if err == nil {
			bricks = append(bricks, loaded...)
			continue
		}
		if errors.Is(err, ErrIndexNotFound) && slices.Contains(errs, err) {
			continue
		}
		errs = append(errs, err)
	}
	return bricks, errors.Join(errs...)
}

func loadReference(index *Index, bricksDir *paths.Path, plat platform.Platform, ref Reference) ([]bricksindex.Brick, error) {
	resolved, err := Resolve(index, bricksDir, []Reference{ref})
	if err != nil {
		return nil, err
	}
	return LoadResolved(plat, resolved)
}

func resolve(index *Index, bricksDir *paths.Path, ref Reference) (ResolvedBrick, error) {
	version := ref.Version
	if version == "" {
		// An unpinned brick sticks to the latest installed version, which changes only when a newer one is installed.
		latest, err := latestInstalled(bricksDir, ref.ID)
		if err != nil {
			return ResolvedBrick{}, err
		}
		version = latest
	}
	if version != "" {
		path, err := InstallPath(bricksDir, ref.ID, version)
		if err != nil {
			return ResolvedBrick{}, err
		}
		if path.IsDir() {
			return resolveInstalled(index, ref, version, path)
		}
	}

	if index == nil {
		return ResolvedBrick{}, ErrIndexNotFound
	}
	indexBrick, release, err := index.FindRelease(ref.ID, version)
	if err != nil {
		return ResolvedBrick{}, err
	}
	path, err := InstallPath(bricksDir, ref.ID, release.Version)
	if err != nil {
		return ResolvedBrick{}, err
	}
	return ResolvedBrick{
		ID:         ref.ID,
		Version:    release.Version,
		Path:       path,
		IndexBrick: indexBrick,
		Release:    release,
	}, nil
}

// resolveInstalled checks an installed version against the index, when there is one: a
// version the index no longer lists may have been removed because it is broken or unsafe.
// Without an index, the installed version is trusted.
func resolveInstalled(index *Index, ref Reference, version string, path *paths.Path) (ResolvedBrick, error) {
	res := ResolvedBrick{ID: ref.ID, Version: version, Path: path, Installed: true}
	// TODO: should the index always be installed?
	if index == nil {
		return res, nil
	}
	indexBrick, release, err := index.FindRelease(ref.ID, version)
	if err != nil {
		hint := ""
		if ref.Version != "" {
			hint = " and pin a version available in the index"
		}
		return ResolvedBrick{}, fmt.Errorf("%w: uninstall it with 'arduino-app-cli brick uninstall %s@%s'%s", err, ref.ID, version, hint)
	}
	res.IndexBrick, res.Release = indexBrick, release
	return res, nil
}

// latestInstalled returns the latest installed version of the brick, or an empty string if none is.
func latestInstalled(bricksDir *paths.Path, id string) (string, error) {
	namespace, name, err := splitBrickID(id)
	if err != nil {
		return "", err
	}
	versions, err := installedVersions(bricksDir.Join(namespace), name)
	if err != nil || len(versions) == 0 {
		return "", err
	}
	return versions[len(versions)-1], nil
}
