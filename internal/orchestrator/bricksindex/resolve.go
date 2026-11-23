// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"

	"github.com/arduino/go-paths-helper"
	"go.bug.st/downloader/v3"
	semver "go.bug.st/relaxed-semver"
)

const BuiltInNamespace = "arduino"

var ErrNotInstalled = errors.New("brick not installed")

// Reference is a brick used by an app: the version is empty when it is not pinned.
type Reference struct {
	ID      string
	Version string
}

// resolvedBrick is the version a third-party reference resolves to.
type resolvedBrick struct {
	version *semver.Version
	// installed is the definition of the version, nil when it is not installed.
	installed  *Brick
	indexBrick *RemoteBrick
	release    *RemoteRelease
}

// IsThirdParty reports whether the brick is not built-in.
func IsThirdParty(id string) bool {
	namespace, _, err := ParseBrickID(id)
	return err == nil && namespace != "" && namespace != BuiltInNamespace
}

// WithRemoteIndex sets the folder of the saved bricks index, which the third-party bricks of an
// app are checked against.
func (b *BricksIndex) WithRemoteIndex(dir *paths.Path) *BricksIndex {
	b.remoteIndexDir = dir
	return b
}

func (b *BricksIndex) loadRemoteIndex() (*RemoteIndex, error) {
	if b.remoteIndexDir == nil {
		return nil, ErrIndexNotFound
	}
	return LoadRemoteIndex(b.remoteIndexDir)
}

// WithApp returns a copy of the index with the bricks local to an app and the third-party bricks
// it uses, in the versions they resolve to. The receiver is not modified, so it can be shared
// between apps. A reference that cannot be resolved is reported in the error and its brick is
// not found in the returned index, which still holds every other brick.
func (b *BricksIndex) WithApp(appBricks []Brick, refs []Reference) (*BricksIndex, error) {
	res := b.withAppBricks(appBricks)
	var errs []error
	var thirdParty []Reference
	for _, ref := range refs {
		if IsThirdParty(ref.ID) {
			thirdParty = append(thirdParty, ref)
		} else if ref.Version != "" {
			errs = append(errs, fmt.Errorf("brick %q cannot be pinned to a version: only third-party bricks can", ref.ID))
		}
	}
	if len(thirdParty) == 0 {
		return res, errors.Join(errs...)
	}

	index, err := b.loadRemoteIndex()
	if err != nil {
		for _, ref := range thirdParty {
			res.unresolved = append(res.unresolved, ref.ID)
		}
		return res, errors.Join(append(errs, err)...)
	}
	for _, ref := range thirdParty {
		r, err := b.resolve(index, ref)
		if err == nil && r.installed == nil {
			err = fmt.Errorf("%w: %s@%s, install it with 'arduino-app-cli brick install %s@%s'", ErrNotInstalled, ref.ID, r.version, ref.ID, r.version)
		}
		if err != nil {
			errs = append(errs, err)
			res.unresolved = append(res.unresolved, ref.ID)
			continue
		}
		res.thirdParty = append(res.thirdParty, *r.installed)
	}
	return res, errors.Join(errs...)
}

// LookupReference resolves a third-party reference. It returns the definition of the brick when
// the version it resolves to is installed, and nil when the saved index lists it but it is not
// installed yet.
func (b *BricksIndex) LookupReference(ref Reference) (*Brick, error) {
	index, err := b.loadRemoteIndex()
	if err != nil {
		return nil, err
	}
	r, err := b.resolve(index, ref)
	if err != nil {
		return nil, err
	}
	return r.installed, nil
}

// InstallReferences installs the third-party bricks the references resolve to that
// are not installed yet, calling progress before each one. The index is downloaded from indexURL
// when there is none on the board, and when it is older than RemoteIndexMaxAge: if that update fails,
// the saved index is used.
func (b *BricksIndex) InstallReferences(ctx context.Context, config downloader.Config, indexURL *url.URL, refs []Reference, install func(*RemoteBrick, *RemoteRelease) error, progress func(string)) error {
	refs = slices.DeleteFunc(slices.Clone(refs), func(r Reference) bool { return !IsThirdParty(r.ID) })
	if len(refs) == 0 {
		return nil
	}
	if b.installedDir == nil || b.remoteIndexDir == nil {
		return errors.New("the bricks index does not include the third-party bricks")
	}
	update := func() (*RemoteIndex, error) {
		progress("updating the bricks index")
		return UpdateRemoteIndex(ctx, config, indexURL, b.remoteIndexDir)
	}

	index, err := b.loadRemoteIndex()
	if err != nil {
		// A broken index is replaced, like a missing one.
		if index, err = update(); err != nil {
			return err
		}
	} else if !RemoteIndexIsRecent(b.remoteIndexDir, RemoteIndexMaxAge) {
		if updated, err := update(); err == nil {
			index = updated
		} else {
			progress(fmt.Sprintf("cannot update the bricks index, using the saved one: %v", err))
		}
	}

	resolved := make([]resolvedBrick, 0, len(refs))
	for _, ref := range refs {
		r, err := b.resolve(index, ref)
		if err != nil {
			return err
		}
		resolved = append(resolved, r)
	}

	for i, r := range resolved {
		if r.installed != nil {
			continue
		}
		progress(fmt.Sprintf("installing the brick %s@%s", refs[i].ID, r.version))
		if err := install(r.indexBrick, r.release); err != nil {
			return err
		}
	}
	return nil
}

// resolve returns the version a third-party reference resolves to, without accessing the network:
//   - a pinned reference resolves to its version;
//   - an unpinned reference resolves to the latest installed version that supports the board or,
//     if there is none, to the latest version of the index that supports it.
//
// An installed version must still be listed in the index: a version the index no longer lists
// may have been removed because it is broken or unsafe.
func (b *BricksIndex) resolve(index *RemoteIndex, ref Reference) (resolvedBrick, error) {
	namespace, name, err := splitBrickID(ref.ID)
	if err != nil {
		return resolvedBrick{}, err
	}
	var res resolvedBrick
	if ref.Version != "" {
		if res.version, err = semver.Parse(ref.Version); err != nil {
			return resolvedBrick{}, fmt.Errorf("invalid version %q for brick %q: %w", ref.Version, ref.ID, err)
		}
		if b.installedDir != nil {
			installed, err := ListInstalledVersions(b.installedDir, ref.ID, res.version)
			if err != nil {
				return resolvedBrick{}, err
			}
			if len(installed) > 0 {
				res.version = installed[len(installed)-1]
				dir := installPath(b.installedDir, ref.ID, res.version)
				brick, err := loadInstalledBrick(b.platform, dir, namespace, name)
				if err != nil {
					return resolvedBrick{}, fmt.Errorf("loading brick %s@%s: %w", ref.ID, res.version, err)
				}
				if !brick.SupportsBoard(b.platform) {
					return resolvedBrick{}, fmt.Errorf("%w: %s@%s does not support %s", ErrUnsupportedBoard, ref.ID, res.version, b.platform.BoardName)
				}
				brick.Version = res.version.String()
				res.installed = &brick
			}
		}
	} else if brick, found := b.findInstalledBrick(ref.ID); found {
		// An unpinned brick sticks to the latest installed version, which changes only when a newer one is installed.
		res.version = semver.MustParse(brick.Version)
		res.installed = &brick
	}

	res.indexBrick, err = index.FindBrick(ref.ID)
	if err == nil {
		res.release, err = res.indexBrick.FindRelease(res.version, b.platform)
	}
	if err != nil {
		if res.installed == nil {
			if errors.Is(err, ErrBrickNotFound) || errors.Is(err, ErrReleaseNotFound) {
				return resolvedBrick{}, fmt.Errorf("%w: if it was published recently, run 'arduino-app-cli brick update-index'", err)
			}
			return resolvedBrick{}, err
		}
		hint := ""
		if ref.Version != "" {
			hint = " and pin a version available in the index"
		}
		return resolvedBrick{}, fmt.Errorf("%w: uninstall it with 'arduino-app-cli brick uninstall %s@%s'%s", err, ref.ID, res.version, hint)
	}
	res.version = res.release.Version
	return res, nil
}
