// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

func TestGetBrickPorts(t *testing.T) {
	index := &bricksindex.BricksIndex{
		BuiltInBricks: []bricksindex.Brick{
			{ID: "arduino:streamlit_ui", Ports: []string{"7000"}, RequiresDisplay: "webview"},
			// A webview brick publishing more than one port: the index declares the display at
			// brick level, so both ports are reported as a webview.
			{ID: "arduino:dashboard", Ports: []string{"7100", "7101"}, RequiresDisplay: "webview"},
			{ID: "arduino:data_logger", Ports: []string{"8080"}},
			{ID: "arduino:object_detection"},
		},
	}

	t.Run("the intent comes from requires_display", func(t *testing.T) {
		got, err := getBrickPorts([]app.Brick{
			{ID: "arduino:streamlit_ui"},
			{ID: "arduino:dashboard"},
			{ID: "arduino:data_logger"},
			{ID: "arduino:object_detection"},
		}, index)

		require.NoError(t, err)
		assert.Equal(t, []brickPorts{
			{brickID: "arduino:streamlit_ui", ports: []string{"7000"}, intent: "webview"},
			{brickID: "arduino:dashboard", ports: []string{"7100", "7101"}, intent: "webview"},
			{brickID: "arduino:data_logger", ports: []string{"8080"}, intent: "user"},
			{brickID: "arduino:object_detection", ports: []string{}, intent: "user"},
		}, got)
	})

	t.Run("a brick missing from the index is an error", func(t *testing.T) {
		_, err := getBrickPorts([]app.Brick{{ID: "arduino:unknown-brick"}}, index)
		assert.ErrorContains(t, err, `brick "arduino:unknown-brick" not found in the index`)
	})
}

func TestBuildAppPortResponse(t *testing.T) {
	testCases := []struct {
		name     string
		appPorts []int
		bricks   []brickPorts
		services []orchestrator.RequiredService
		want     []port
	}{
		{
			name: "an app with no port at all",
			want: []port{},
		},
		{
			name:     "the app.yaml ports are for the user",
			appPorts: []int{7000},
			want:     []port{{Port: "7000", Source: "app.yaml", ServiceName: "user"}},
		},
		{
			name:   "the brick ports keep the intent of the brick",
			bricks: []brickPorts{{brickID: "arduino:streamlit_ui", ports: []string{"7000"}, intent: "webview"}},
			want:   []port{{Port: "7000", Source: "arduino:streamlit_ui", ServiceName: "webview"}},
		},
		{
			name: "the service ports are internal and reported against the service",
			services: []orchestrator.RequiredService{
				{ID: "arduino:audio", Name: "audio service", BrickID: "arduino:asr", Ports: []string{"8085"}},
			},
			want: []port{{Port: "8085", Source: "arduino:audio", ServiceName: "internal"}},
		},
		{
			name:     "the app ports come first, the service ones last",
			appPorts: []int{7000},
			bricks: []brickPorts{
				{brickID: "arduino:asr", ports: []string{}, intent: "user"},
				{brickID: "arduino:streamlit_ui", ports: []string{"7100"}, intent: "webview"},
			},
			services: []orchestrator.RequiredService{
				{ID: "arduino:audio", Name: "audio service", BrickID: "arduino:asr", Ports: []string{"8085"}},
			},
			want: []port{
				{Port: "7000", Source: "app.yaml", ServiceName: "user"},
				{Port: "7100", Source: "arduino:streamlit_ui", ServiceName: "webview"},
				{Port: "8085", Source: "arduino:audio", ServiceName: "internal"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildAppPortResponse(tc.appPorts, tc.bricks, tc.services)
			assert.Equal(t, tc.want, got.Ports)
		})
	}
}
