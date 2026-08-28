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

func TestParseSimulatedUpgradeOutput(t *testing.T) {
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
				name:  "package without the installed version",
				input: "Inst nano (2.9.3-2 Ubuntu:18.04/bionic-updates [amd64])",
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
				input: "Inst apt [2.0.10] (2.0.11 Ubuntu:20.04/focal-updates [amd64])",
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
				name:  "architecture qualified name",
				input: "Inst libgcc-s1:i386 [10.5.0-1] (10.5.0-4 Debian:13/trixie [i386])",
				expected: []update.UpgradablePackage{
					{
						Type:         update.Debian,
						Name:         "libgcc-s1",
						ToVersion:    "10.5.0-4",
						FromVersion:  "10.5.0-1",
						Architecture: "i386",
					},
				},
			},
			{
				name: "multiple packages",
				input: `Reading package lists...
Building dependency tree...
Reading state information...
Calculating upgrade...
The following packages will be upgraded:
  apt code containerd.io distro-info-data
4 upgraded, 0 newly installed, 0 to remove and 0 not upgraded.
Inst distro-info-data [0.43ubuntu1.16] (0.43ubuntu1.18 Ubuntu:20.04/focal-updates [all])
Inst apt [2.0.10] (2.0.11 Ubuntu:20.04/focal-updates [amd64]) []
Conf apt (2.0.11 Ubuntu:20.04/focal-updates [amd64])
Inst code [1.100.2-1747260578] (1.100.3-1748872405 code:stable [amd64])
Inst containerd.io [1.7.25-1] (1.7.27-1 Docker:focal [amd64])
Conf containerd.io (1.7.27-1 Docker:focal [amd64])
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
			{
				// A held back package has no Inst line: it must not be listed, because
				// apt-get install --only-upgrade would then fail on it.
				name: "held back package",
				input: `Reading package lists...
Building dependency tree...
Reading state information...
Calculating upgrade...
The following packages have been kept back:
  alsa-ucm-conf libasound2t64
The following packages will be upgraded:
  arduino-app-cli
1 upgraded, 0 newly installed, 0 to remove and 2 not upgraded.
Inst arduino-app-cli [1.2.0] (1.3.0 arduino:stable [arm64])
Conf arduino-app-cli (1.3.0 arduino:stable [arm64])
`,
				expected: []update.UpgradablePackage{
					{
						Type:         update.Debian,
						Name:         "arduino-app-cli",
						ToVersion:    "1.3.0",
						FromVersion:  "1.2.0",
						Architecture: "arm64",
					},
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				res := parseSimulatedUpgradeOutput(strings.NewReader(tt.input))
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
				Source:  orchestrator.InitSourceDocker,
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
				Source:  orchestrator.InitSourceDocker,
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
