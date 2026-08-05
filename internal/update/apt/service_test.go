// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package apt

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
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

func TestParseSystemInitLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected orchestrator.InitResult
	}{
		{
			name:  "progress event",
			input: `{"type":"progress","source":"docker","label":"arduino/app:1.0","current":50,"total":100,"percent":50}`,
			expected: orchestrator.InitResult{
				Type:    orchestrator.InitResultProgress,
				Source:  "docker",
				Label:   "arduino/app:1.0",
				Current: 50,
				Total:   100,
				Percent: 50,
			},
		},
		{
			name:  "log event",
			input: `{"type":"log","source":"docker","message":"pulling images"}`,
			expected: orchestrator.InitResult{
				Type:    orchestrator.InitResultLog,
				Source:  "docker",
				Message: "pulling images",
			},
		},
		{
			// Standard error is redirected to the same stream and is not structured.
			name:     "line that is not json",
			input:    "time=2026-08-05 level=WARN msg=something happened",
			expected: orchestrator.InitResult{Type: orchestrator.InitResultLog, Message: "time=2026-08-05 level=WARN msg=something happened"},
		},
		{
			name:     "json without a type",
			input:    `{"message":"orphan"}`,
			expected: orchestrator.InitResult{Type: orchestrator.InitResultLog, Message: `{"message":"orphan"}`},
		},
		{
			name:     "empty line",
			input:    "",
			expected: orchestrator.InitResult{Type: orchestrator.InitResultLog, Message: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, parseSystemInitLine(tt.input))
		})
	}
}

func TestImagesDownloadBand(t *testing.T) {
	require.Equal(t, imagesDownloadProgress, imagesDownloadBand(0))
	require.Equal(t, imagesCleanupProgress, imagesDownloadBand(100))
	require.Equal(t, float32(60), imagesDownloadBand(50))

	// The percentage comes from another process: out of range values must not push
	// the progress outside of the band.
	require.Equal(t, imagesDownloadProgress, imagesDownloadBand(-10))
	require.Equal(t, imagesCleanupProgress, imagesDownloadBand(300))
}
