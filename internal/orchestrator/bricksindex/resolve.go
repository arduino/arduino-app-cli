// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"fmt"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

// BuiltInNamespace is the namespace of the bricks shipped with the cli. For now they are
// looked up only in the built-in index.
const BuiltInNamespace = "arduino"

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
	// IndexBrick and Release are the entries of the saved index.
	IndexBrick *Brick
	Release    *Release
}

// IsThirdParty reports whether the brick is not built-in.
func IsThirdParty(id string) bool {
	namespace, _, err := bricksindex.ParseBrickID(id)
	return err == nil && namespace != "" && namespace != BuiltInNamespace
}

// Resolve returns the version each third-party reference resolves to, without accessing the network:
//   - a pinned reference resolves to its version, which must be listed in the saved index;
//   - an unpinned reference resolves to the latest version of the saved index.
//
// index is nil when it was never downloaded, which is an error only if a third-party brick is referenced. References that are not third-party are skipped,
// unless they are pinned, which is an error. Bricks local to the app must be filtered out
// by the caller.
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
			return nil, fmt.Errorf("brick %s@%s is not installed, install it with 'arduino-app-cli brick install %s@%s'", r.ID, r.Version, r.ID, r.Version)
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

func resolve(index *Index, bricksDir *paths.Path, ref Reference) (ResolvedBrick, error) {
	if index == nil {
		return ResolvedBrick{}, ErrIndexNotFound
	}
	indexBrick, release, findErr := index.FindRelease(ref.ID, ref.Version)
	if findErr != nil {
		// The release may have been removed from the index because it is broken or unsafe.
		if ref.Version != "" {
			if path, err := InstallPath(bricksDir, ref.ID, ref.Version); err == nil && path.IsDir() {
				return ResolvedBrick{}, fmt.Errorf("%w: uninstall it with 'arduino-app-cli brick uninstall %s@%s' and pin a version available in the index",
					findErr, ref.ID, ref.Version)
			}
		}
		return ResolvedBrick{}, findErr
	}

	path, err := InstallPath(bricksDir, ref.ID, release.Version)
	if err != nil {
		return ResolvedBrick{}, err
	}
	return ResolvedBrick{
		ID:         ref.ID,
		Version:    release.Version,
		Path:       path,
		Installed:  path.IsDir(),
		IndexBrick: indexBrick,
		Release:    release,
	}, nil
}
