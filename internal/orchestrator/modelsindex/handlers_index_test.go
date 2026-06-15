// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package modelsindex

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDownloadHandlerLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected ModelDownloadEvent
	}{
		{
			name: "start event",
			line: `{"event":"start","description":"downloading"}`,
			expected: ModelDownloadEvent{
				Type:        ModelInstallEventStart,
				Description: "downloading",
			},
		},
		{
			name: "update event",
			line: `{"event":"update","current":64,"total":128,"unit":"bytes","percentage":"50%"}`,
			expected: ModelDownloadEvent{
				Type:       ModelInstallEventUpdate,
				Current:    64,
				Total:      128,
				Unit:       "bytes",
				Percentage: "50%",
			},
		},
		{
			name: "complete event with artifacts",
			line: `{"event":"complete","artifacts":["model.eim","meta.json"]}`,
			expected: ModelDownloadEvent{
				Type:      ModelInstallEventComplete,
				Artifacts: []string{"model.eim", "meta.json"},
			},
		},
		{
			name: "error event",
			line: `{"event":"error","description":"network failure"}`,
			expected: ModelDownloadEvent{
				Type:        ModelInstallEventError,
				Description: "network failure",
			},
		},
		{
			name: "stat event maps to info and converts size_mb",
			line: `{"event":"stat","size_mb":1.5}`,
			expected: ModelDownloadEvent{
				Type:  ModelInstallEventInfo,
				Total: int64(1.5 * 1024 * 1024),
			},
		},
		{
			name: "unknown event maps to info",
			line: `{"event":"something-else","description":"note"}`,
			expected: ModelDownloadEvent{
				Type:        ModelInstallEventInfo,
				Description: "note",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []ModelDownloadEvent

			parseDownloadHandlerLine(tt.line, func(e ModelDownloadEvent) {
				got = append(got, e)
			})

			require.Len(t, got, 1)
			assert.Equal(t, tt.expected, got[0])
		})
	}

	t.Run("invalid json does not publish", func(t *testing.T) {
		called := false

		parseDownloadHandlerLine("not-json", func(ModelDownloadEvent) {
			called = true
		})

		assert.False(t, called)
	})
}