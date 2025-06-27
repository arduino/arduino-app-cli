package orchestrator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseListUpgradableOutput(t *testing.T) {
	t.Run("edges cases", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected []UpgradablePackage
		}{
			{
				name:     "empty input",
				input:    "",
				expected: []UpgradablePackage{},
			},
			{
				name:     "line not matching regex",
				input:    "this-is-not a-valid-line\n",
				expected: []UpgradablePackage{},
			},
			{
				name:  "upgradable package without [upgradable from]",
				input: "nano/bionic-updates 2.9.3-2 amd64",
				expected: []UpgradablePackage{
					{
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
				expected: []UpgradablePackage{
					{
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
				expected: []UpgradablePackage{
					{
						Name:         "distro-info-data",
						ToVersion:    "0.43ubuntu1.18",
						FromVersion:  "0.43ubuntu1.16",
						Architecture: "all",
					},
					{
						Name:         "apt",
						ToVersion:    "2.0.11",
						FromVersion:  "2.0.10",
						Architecture: "amd64",
					},
					{
						Name:         "code",
						ToVersion:    "1.100.3-1748872405",
						FromVersion:  "1.100.2-1747260578",
						Architecture: "amd64",
					},
					{
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
