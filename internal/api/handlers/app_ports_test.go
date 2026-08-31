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
// and in particular that the serviceName field keeps reporting requires_display: the consumers
// reading it compare it to the "webview" literal.
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
				{Port: "7000", Source: "arduino:web_ui", RequiresDisplay: "webview"},
			},
			want: []port{{Port: "7000", Source: "arduino:web_ui", ServiceName: "webview"}},
		},
		{
			name: "a port declared by the app.yaml has no service name",
			ports: []orchestrator.PortInfo{
				{Port: "7000", Source: "app.yaml"},
			},
			want: []port{{Port: "7000", Source: "app.yaml"}},
		},
		{
			name: "a port of a required service has no service name",
			ports: []orchestrator.PortInfo{
				{Port: "8085", Source: "arduino:genie_audio"},
			},
			want: []port{{Port: "8085", Source: "arduino:genie_audio"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, buildAppPortResponse(tc.ports).Ports)
		})
	}
}
