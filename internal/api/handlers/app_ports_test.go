// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

// TestBuildAppPortResponse checks the mapping of the ports of an app to the response of the API,
// and in particular that the deprecated serviceName field keeps reporting requires_display: the
// consumers reading it compare it to the "webview" literal.
func TestBuildAppPortResponse(t *testing.T) {
	testCases := []struct {
		name  string
		ports []orchestrator.PortInfo
		want  []port
	}{
		{
			name:  "an app exposing no port",
			ports: []orchestrator.PortInfo{},
			want:  []port{},
		},
		{
			name: "a webview port keeps reporting requires_display",
			ports: []orchestrator.PortInfo{
				{Port: "7000", Source: "arduino:web_ui", Intent: orchestrator.WebviewIntent, RequiresDisplay: "webview"},
			},
			want: []port{{Port: "7000", Source: "arduino:web_ui", Intent: "webview", ServiceName: "webview"}},
		},
		{
			name: "a port of the user has no service name",
			ports: []orchestrator.PortInfo{
				{Port: "7000", Source: "app.yaml", Intent: orchestrator.UserIntent},
			},
			want: []port{{Port: "7000", Source: "app.yaml", Intent: "user"}},
		},
		{
			name: "an internal port has no service name",
			ports: []orchestrator.PortInfo{
				{Port: "8085", Source: "arduino:genie_audio", Intent: orchestrator.InternalIntent},
			},
			want: []port{{Port: "8085", Source: "arduino:genie_audio", Intent: "internal"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, buildAppPortResponse(tc.ports).Ports)
		})
	}
}
