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

func TestResolveVolumes(t *testing.T) {
	t.Run("it resolves variables and adds env only once", func(t *testing.T) {
		vols := []string{
			"${MODELS_DIR}:/models",
			"${MODELS_DIR}:/backup",
			"${CACHE_DIR:-/tmp/cache}:/cache",
		}
		vars := map[string]string{
			"MODELS_DIR": "/var/models",
		}

		binds, envAdditions := ResolveVolumes(vols, vars)

		assert.Equal(t, []string{
			"/var/models:/models",
			"/var/models:/backup",
			"/tmp/cache:/cache",
		}, binds)
		assert.Equal(t, []string{"MODELS_DIR=/var/models"}, envAdditions)
	})

	t.Run("it uses provided value instead of default and tracks multiple variables", func(t *testing.T) {
		vols := []string{
			"${MODELS_DIR:-/tmp/models}:/models",
			"${CACHE_DIR:-/tmp/cache}:/cache",
		}
		vars := map[string]string{
			"MODELS_DIR": "/opt/models",
			"CACHE_DIR":  "/opt/cache",
		}

		binds, envAdditions := ResolveVolumes(vols, vars)

		assert.Equal(t, []string{
			"/opt/models:/models",
			"/opt/cache:/cache",
		}, binds)
		assert.Equal(t, []string{"MODELS_DIR=/opt/models", "CACHE_DIR=/opt/cache"}, envAdditions)
	})

	t.Run("it leaves empty substitution when variable is missing and no default", func(t *testing.T) {
		vols := []string{"${UNSET_VAR}:/models"}

		binds, envAdditions := ResolveVolumes(vols, map[string]string{})

		assert.Equal(t, []string{":/models"}, binds)
		assert.Empty(t, envAdditions)
	})
}
