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

package apt

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/update"
)

func TestParseListUpgradableOutput(t *testing.T) {
	t.Run("edges cases", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected []update.UpgradablePackage
		}{
			{
				name:     "empty input",
				input:    "",
				expected: []update.UpgradablePackage{},
			},
			{
				name:     "line not matching regex",
				input:    "this-is-not a-valid-line\n",
				expected: []update.UpgradablePackage{},
			},
			{
				name:  "upgradable package without [upgradable from]",
				input: "nano/bionic-updates 2.9.3-2 amd64",
				expected: []update.UpgradablePackage{
					{
						Type:         update.Debian,
						Name:         "nano",
						ToVersion:    "2.9.3-2",
						FromVersion:  "",
						Architecture: "amd64",
					},
				},
			},
			{
				name:  "package with from and to versions",
				input: "apt/focal-updates 2.0.11 amd64 [upgradable from: 2.0.10]",
				expected: []update.UpgradablePackage{
					{
						Type:         update.Debian,
						Name:         "apt",
						ToVersion:    "2.0.11",
						FromVersion:  "2.0.10",
						Architecture: "amd64",
					},
				},
			},
			{
				name: "multiple packages",
				input: `
distro-info-data/focal-updates,focal-updates 0.43ubuntu1.18 all [upgradable from: 0.43ubuntu1.16]
apt/focal-updates 2.0.11 amd64 [upgradable from: 2.0.10]
code/stable 1.100.3-1748872405 amd64 [upgradable from: 1.100.2-1747260578]
containerd.io/focal 1.7.27-1 amd64 [upgradable from: 1.7.25-1]
`,
				expected: []update.UpgradablePackage{
					{
						Type:         update.Debian,
						Name:         "distro-info-data",
						ToVersion:    "0.43ubuntu1.18",
						FromVersion:  "0.43ubuntu1.16",
						Architecture: "all",
					},
					{
						Type:         update.Debian,
						Name:         "apt",
						ToVersion:    "2.0.11",
						FromVersion:  "2.0.10",
						Architecture: "amd64",
					},
					{
						Type:         update.Debian,
						Name:         "code",
						ToVersion:    "1.100.3-1748872405",
						FromVersion:  "1.100.2-1747260578",
						Architecture: "amd64",
					},
					{
						Type:         update.Debian,
						Name:         "containerd.io",
						ToVersion:    "1.7.27-1",
						FromVersion:  "1.7.25-1",
						Architecture: "amd64",
					},
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				res := parseListUpgradableOutput(strings.NewReader(tt.input))
				require.Equal(t, tt.expected, res)
			})
		}
	})
}
