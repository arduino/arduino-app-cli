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
	"net/url"
	"slices"

	"github.com/arduino/go-paths-helper"
	"go.bug.st/downloader/v3"
	semver "go.bug.st/relaxed-semver"

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
	Version *semver.Version
	// Path is where the version is installed, or will be.
	Path      *paths.Path
	Installed bool
	// IndexBrick and Release are the entries of the saved index. They are nil when the
	// version is installed and there is no saved index.
	IndexBrick *RemoteBrick
	Release    *RemoteRelease
}

// IsThirdParty reports whether the brick is not built-in.
func IsThirdParty(id string) bool {
	namespace, _, err := ParseBrickID(id)
	return err == nil && namespace != "" && namespace != BuiltInNamespace
}

// Resolve returns the version each third-party reference resolves to, without accessing the network:
//   - a pinned reference resolves to its version;
//   - an unpinned reference resolves to the latest installed version or, if none is
//     installed, to the latest version of the saved index that supports the board of plat.
func Resolve(index *RemoteIndex, bricksDir *paths.Path, plat platform.Platform, refs []Reference) ([]ResolvedBrick, error) {
	var res []ResolvedBrick
	for _, ref := range refs {
		if !IsThirdParty(ref.ID) {
			if ref.Version != "" {
				return nil, fmt.Errorf("brick %q cannot be pinned to a version: only third-party bricks can", ref.ID)
			}
			continue
		}
		resolved, err := resolve(index, bricksDir, plat, ref)
		if err != nil {
			return nil, err
		}
		res = append(res, resolved)
	}
	return res, nil
}

// LoadResolved loads the definitions of the resolved bricks for the platform. Every brick
// must be installed and support the board's platform.
func LoadResolved(plat platform.Platform, resolved []ResolvedBrick) ([]Brick, error) {
	bricks := make([]Brick, 0, len(resolved))
	for _, r := range resolved {
		if !r.Installed {
			return nil, fmt.Errorf("%w: %s@%s, install it with 'arduino-app-cli brick install %s@%s'", ErrNotInstalled, r.ID, r.Version, r.ID, r.Version)
		}
		namespace, name, _ := ParseBrickID(r.ID) // resolve checks the id
		brick, err := loadInstalledBrick(plat, r.Path, namespace, name)
		if err != nil {
			return nil, fmt.Errorf("loading brick %s@%s: %w", r.ID, r.Version, err)
		}
		brick.Version = r.Version.String()
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

// Install installs, with install, the third-party bricks the references resolve to that are not
// installed yet, calling progress before each one. The index is updated when there is none on the
// board, even if every brick is installed, and when it does not list a brick or its version.
func (s Source) Install(ctx context.Context, config downloader.Config, refs []Reference, install func(*RemoteBrick, *RemoteRelease) error, progress func(string)) error {
	if !slices.ContainsFunc(refs, func(r Reference) bool { return IsThirdParty(r.ID) }) {
		return nil
	}
	update := func() (*RemoteIndex, error) {
		progress("updating the bricks index")
		return UpdateRemoteIndex(ctx, config, s.IndexURL, s.IndexDir)
	}

	index, err := LoadRemoteIndex(s.IndexDir)
	updated := err != nil
	if updated {
		// A broken index is replaced, like a missing one.
		slog.Debug("cannot load the bricks index", "error", err)
		if index, err = update(); err != nil {
			return err
		}
	}
	resolved, err := Resolve(index, s.BricksDir, s.Platform, refs)
	if !updated && (errors.Is(err, ErrBrickNotFound) || errors.Is(err, ErrReleaseNotFound)) {
		updatedIndex, updateErr := update()
		if updateErr != nil {
			return errors.Join(err, updateErr)
		}
		resolved, err = Resolve(updatedIndex, s.BricksDir, s.Platform, refs)
	}
	if err != nil {
		return err
	}

	for _, r := range resolved {
		if r.Installed {
			continue
		}
		progress(fmt.Sprintf("installing the brick %s@%s", r.ID, r.Version))
		if err := install(r.IndexBrick, r.Release); err != nil {
			return err
		}
	}
	return nil
}

// Lookup resolves a third-party reference. It returns the definition of the brick when the version it resolves to is
// installed, and nil when the saved index lists it but it is not installed yet.
func (s Source) Lookup(ref Reference) (*Brick, error) {
	index, err := LoadRemoteIndex(s.IndexDir)
	if err != nil && (s.RequireIndex || !errors.Is(err, ErrIndexNotFound)) {
		return nil, err
	}
	resolved, err := Resolve(index, s.BricksDir, s.Platform, []Reference{ref})
	if err != nil {
		return nil, err
	}
	if len(resolved) == 0 || !resolved[0].Installed {
		return nil, nil
	}
	bricks, err := LoadResolved(s.Platform, resolved)
	if err != nil {
		return nil, err
	}
	return &bricks[0], nil
}

// Load resolves the references against the saved index and loads the installed bricks
// they resolve to, skipping the references that are not third-party. Each brick is
// resolved on its own, so one that fails does not hide the others: on error, the bricks
// that could be loaded are still returned.
func (s Source) Load(refs []Reference) ([]Brick, error) {
	if !slices.ContainsFunc(refs, func(r Reference) bool { return IsThirdParty(r.ID) || r.Version != "" }) {
		return nil, nil
	}
	index, err := LoadRemoteIndex(s.IndexDir)
	if err != nil && (s.RequireIndex || !errors.Is(err, ErrIndexNotFound)) {
		return nil, err
	}

	var bricks []Brick
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

func loadReference(index *RemoteIndex, bricksDir *paths.Path, plat platform.Platform, ref Reference) ([]Brick, error) {
	resolved, err := Resolve(index, bricksDir, plat, []Reference{ref})
	if err != nil {
		return nil, err
	}
	return LoadResolved(plat, resolved)
}

func resolve(index *RemoteIndex, bricksDir *paths.Path, plat platform.Platform, ref Reference) (ResolvedBrick, error) {
	namespace, name, err := splitBrickID(ref.ID)
	if err != nil {
		return ResolvedBrick{}, err
	}
	var version *semver.Version
	if ref.Version != "" {
		if version, err = semver.Parse(ref.Version); err != nil {
			return ResolvedBrick{}, fmt.Errorf("invalid version %q for brick %q: %w", ref.Version, ref.ID, err)
		}
	} else {
		// An unpinned brick sticks to the latest installed version, which changes only when a newer one is installed.
		versions, err := installedVersions(bricksDir.Join(namespace, name))
		if err != nil {
			return ResolvedBrick{}, err
		}
		if len(versions) > 0 {
			version = versions[len(versions)-1]
		}
	}
	if version != nil {
		if path := installPath(bricksDir, ref.ID, version); path.IsDir() {
			return resolveInstalled(index, ref, version, path)
		}
	}

	if index == nil {
		return ResolvedBrick{}, ErrIndexNotFound
	}
	indexBrick, err := index.FindBrick(ref.ID)
	if err != nil {
		return ResolvedBrick{}, err
	}
	release, err := indexBrick.FindRelease(version, plat)
	if err != nil {
		return ResolvedBrick{}, err
	}
	return ResolvedBrick{
		ID:         ref.ID,
		Version:    release.Version,
		Path:       installPath(bricksDir, ref.ID, release.Version),
		IndexBrick: indexBrick,
		Release:    release,
	}, nil
}

// resolveInstalled checks an installed version against the index, when there is one: a
// version the index no longer lists may have been removed because it is broken or unsafe.
// Without an index, the installed version is trusted.
func resolveInstalled(index *RemoteIndex, ref Reference, version *semver.Version, path *paths.Path) (ResolvedBrick, error) {
	res := ResolvedBrick{ID: ref.ID, Version: version, Path: path, Installed: true}
	// TODO: should the index always be installed?
	if index == nil {
		return res, nil
	}
	hint := ""
	if ref.Version != "" {
		hint = " and pin a version available in the index"
	}
	indexBrick, err := index.FindBrick(ref.ID)
	if err != nil {
		return ResolvedBrick{}, fmt.Errorf("%w: uninstall it with 'arduino-app-cli brick uninstall %s@%s'%s", err, ref.ID, version, hint)
	}
	// The board is checked when the brick is loaded, so any board is accepted here.
	release, err := indexBrick.FindRelease(version, platform.Platform{})
	if err != nil {
		return ResolvedBrick{}, fmt.Errorf("%w: uninstall it with 'arduino-app-cli brick uninstall %s@%s'%s", err, ref.ID, version, hint)
	}
	res.IndexBrick, res.Release = indexBrick, release
	return res, nil
}
