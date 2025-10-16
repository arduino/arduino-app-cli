// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package orchestrator

import (
	"encoding/json"
	"fmt"
	"strings"

	semver "go.bug.st/relaxed-semver"
)

// LibraryReleaseID represents a library release identifier in the form of:
// - name[@version]
// Version is optional, if not provided, the latest version available will be used.
type LibraryReleaseID struct {
	Name    string
	Version string
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

// MarshalJSON implements the json.Marshaler interface for LibraryID.
func (l LibraryReleaseID) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.String())
}
