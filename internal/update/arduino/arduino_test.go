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

package arduino

import (
	"testing"

	"github.com/stretchr/testify/require"

	semver "go.bug.st/relaxed-semver"
)

func TestSelectBestVersion(t *testing.T) {
	tests := []struct {
		name        string
		available   []string
		installed   string
		constraint  string
		expectedVer string
		expectNil   bool
	}{
		{
			name:        "Standard upgrade within constraint",
			available:   []string{"1.0.0", "1.1.0", "1.2.0"},
			installed:   "1.0.0",
			constraint:  "^1.0.0",
			expectedVer: "1.2.0",
			expectNil:   false,
		},
		{
			name:        "Upgrade available but blocked by constraint",
			available:   []string{"2.0.0"},
			installed:   "1.0.0",
			constraint:  "^1.0.0",
			expectedVer: "",
			expectNil:   true,
		},
		{
			name:        "Major upgrade allowed by constraint (<3.0.0)",
			available:   []string{"2.0.0"},
			installed:   "1.0.0",
			constraint:  "<3.0.0",
			expectedVer: "2.0.0",
			expectNil:   false,
		},
		{
			name:        "Sorts correctly mixed versions",
			available:   []string{"1.5.0", "1.1.0", "1.9.0", "1.2.0"},
			installed:   "1.0.0",
			constraint:  "^1.0.0",
			expectedVer: "1.9.0",
			expectNil:   false,
		},
		{
			name:        "Ignores older versions",
			available:   []string{"0.9.0", "0.8.0"},
			installed:   "1.0.0",
			constraint:  "^1.0.0",
			expectedVer: "",
			expectNil:   true,
		},
		{
			name:        "Ignores invalid strings",
			available:   []string{"1.1.0", "not-a-version", "invalid"},
			installed:   "1.0.0",
			constraint:  "^1.0.0",
			expectedVer: "1.1.0",
			expectNil:   false,
		},
		{
			name:        "Empty available list returns nil",
			available:   []string{},
			installed:   "1.0.0",
			constraint:  "^1.0.0",
			expectedVer: "",
			expectNil:   true,
		},
		{
			name:        "Includes installed version if present in available",
			available:   []string{"1.0.0"},
			installed:   "1.0.0",
			constraint:  "^1.0.0",
			expectedVer: "1.0.0",
			expectNil:   false,
		},
		{
			name:        "No upgrade found (all available are older)",
			available:   []string{"1.0.0", "1.1.0"},
			installed:   "1.5.0",
			constraint:  "^1.0.0",
			expectedVer: "",
			expectNil:   true,
		},
		{
			name:        "Sorts RC versions correctly",
			available:   []string{"1.1.0", "1.2.0-rc.3", "2.2.0-rc.4"},
			installed:   "1.0.0",
			constraint:  "<2.0.0",
			expectedVer: "1.2.0-rc.3",
			expectNil:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installedV, err := semver.Parse(tt.installed)
			require.NoError(t, err, "Setup: failed to parse installed version")

			constraint, err := semver.ParseConstraint(tt.constraint)
			require.NoError(t, err, "Setup: failed to parse constraint")

			got := selectBestVersion(tt.available, installedV, constraint)

			if tt.expectNil {
				require.Nil(t, got, "Expected result to be nil")
			} else {
				require.NotNil(t, got, "Expected result not to be nil")
				require.Equal(t, tt.expectedVer, got.String(), "Selected version mismatch")
			}
		})
	}
}
