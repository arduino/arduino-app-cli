// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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
