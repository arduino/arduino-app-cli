// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

func TestAppPorts(t *testing.T) {
	servicesIndex := writeServicesIndex(t, map[string]string{
		"arduino:audio": "services:\n  audio:\n    image: busybox\n    ports: [\"8085:8085\"]\n",
		"arduino:db":    "services:\n  db:\n    image: busybox\n    ports: [\"9999:9999\"]\n",
	})

	bIndex := &bricksindex.BricksIndex{
		BuiltInBricks: []bricksindex.Brick{
			{ID: "arduino:web_ui", Ports: []string{"7000"}, RequiresDisplay: "webview"},
			// A webview brick publishing more than one port: the index declares the display at
			// brick level, so both ports are webview ones.
			{ID: "arduino:dashboard", Ports: []string{"7100", "7101"}, RequiresDisplay: "webview"},
			{ID: "arduino:data_logger", Ports: []string{"8080"}},
			{ID: "arduino:object_detection"},
			// Both speech bricks require the same service, like arduino:asr and arduino:tts do.
			{ID: "arduino:asr", RequiresServices: bricksindex.RequiresServices{{ID: "arduino:audio"}}},
			{ID: "arduino:tts", RequiresServices: bricksindex.RequiresServices{{ID: "arduino:audio"}}},
			{ID: "arduino:storage", RequiresServices: bricksindex.RequiresServices{{ID: "arduino:db"}}},
		},
	}

	testCases := []struct {
		name     string
		appPorts []int
		bricks   []app.Brick
		want     []PortInfo
	}{
		{
			name:   "an app exposing no port at all",
			bricks: []app.Brick{{ID: "arduino:object_detection"}},
			want:   []PortInfo{},
		},
		{
			name:     "the app.yaml ports are for the user",
			appPorts: []int{7000},
			want:     []PortInfo{{Port: "7000", Source: "app.yaml", SourceType: AppSourceType, Intent: UserIntent}},
		},
		{
			name:   "every port of a brick requiring a display is a webview one",
			bricks: []app.Brick{{ID: "arduino:dashboard"}},
			want: []PortInfo{
				{Port: "7100", Source: "arduino:dashboard", SourceType: BrickSourceType, Intent: WebviewIntent, RequiresDisplay: "webview"},
				{Port: "7101", Source: "arduino:dashboard", SourceType: BrickSourceType, Intent: WebviewIntent, RequiresDisplay: "webview"},
			},
		},
		{
			name:   "the ports of a brick requiring no display are for the user",
			bricks: []app.Brick{{ID: "arduino:data_logger"}},
			want:   []PortInfo{{Port: "8080", Source: "arduino:data_logger", SourceType: BrickSourceType, Intent: UserIntent}},
		},
		{
			name:   "the ports of a required service are internal and reported against the service",
			bricks: []app.Brick{{ID: "arduino:asr"}},
			want:   []PortInfo{{Port: "8085", Source: "arduino:audio", SourceType: ServiceSourceType, Intent: InternalIntent}},
		},
		{
			name:   "a service required by two bricks publishes its ports once",
			bricks: []app.Brick{{ID: "arduino:asr"}, {ID: "arduino:tts"}},
			want:   []PortInfo{{Port: "8085", Source: "arduino:audio", SourceType: ServiceSourceType, Intent: InternalIntent}},
		},
		{
			name:     "the app ports come first, the service ones last",
			appPorts: []int{6000},
			bricks:   []app.Brick{{ID: "arduino:storage"}, {ID: "arduino:web_ui"}},
			want: []PortInfo{
				{Port: "6000", Source: "app.yaml", SourceType: AppSourceType, Intent: UserIntent},
				{Port: "7000", Source: "arduino:web_ui", SourceType: BrickSourceType, Intent: WebviewIntent, RequiresDisplay: "webview"},
				{Port: "9999", Source: "arduino:db", SourceType: ServiceSourceType, Intent: InternalIntent},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			a := app.ArduinoApp{Descriptor: app.AppDescriptor{Ports: tc.appPorts, Bricks: tc.bricks}}

			got, err := AppPorts(a, bIndex, servicesIndex)

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	t.Run("a brick missing from the index is an error", func(t *testing.T) {
		a := app.ArduinoApp{Descriptor: app.AppDescriptor{Bricks: []app.Brick{{ID: "arduino:unknown-brick"}}}}

		_, err := AppPorts(a, bIndex, servicesIndex)

		assert.ErrorContains(t, err, `brick "arduino:unknown-brick" not found in the index`)
	})
}
