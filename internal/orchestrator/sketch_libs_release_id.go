// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"fmt"
	"strings"

	semver "go.bug.st/relaxed-semver"
)

// LibraryReleaseID represents a library release identifier in the form of:
// - name[@version]
// Version is optional, if not provided, the latest version available will be used.
type LibraryReleaseID struct {
	Name         string
	Version      string
	IsDependency bool
}

func NewLibraryReleaseID(name string, version string) LibraryReleaseID {
	return LibraryReleaseID{
		Name:    name,
		Version: version,
	}
}

func ParseLibraryReleaseID(s string) (LibraryReleaseID, error) {
	split := strings.SplitN(s, "@", 2)

	if len(split) == 1 {
		// No version provided, return the latest version
		return LibraryReleaseID{Name: s}, nil
	}

	if split[1] == "" {
		return LibraryReleaseID{}, fmt.Errorf("missing version")
	}
	if _, err := semver.Parse(split[1]); err != nil {
		return LibraryReleaseID{}, err
	}

	return LibraryReleaseID{Name: split[0], Version: split[1]}, nil
}

func (l LibraryReleaseID) String() string {
	if l.Version == "" {
		return l.Name
	}
	return l.Name + "@" + l.Version
}
