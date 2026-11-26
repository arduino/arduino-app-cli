// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

func TestMissingBricks(t *testing.T) {
	bricksDir := paths.New(t.TempDir())
	brickDir := bricksDir.Join("acme", "camera", "1.0.0", "src", "camera")
	require.NoError(t, brickDir.MkdirAll())
	require.NoError(t, brickDir.Join("brick_config.yaml").WriteFile([]byte("id: acme:camera\n")))

	a := app.ArduinoApp{
		Descriptor: app.AppDescriptor{Bricks: []app.Brick{
			{ID: "arduino:web_ui"},
			{ID: "arduino:published"},
			{ID: "acme:local"},
			{ID: "acme:camera"},
			{ID: "acme:camera", Version: "1.0.0"},
			{ID: "acme:camera", Version: "2.0.0"},
			{ID: "acme:audio"},
		}},
		LocalBricks: []bricksindex.Brick{{ID: "acme:local"}},
	}
	index := (&bricksindex.BricksIndex{BuiltInBricks: []bricksindex.Brick{{ID: "arduino:web_ui"}}}).WithInstalledBricks(bricksDir)

	// The built-in and the local bricks are never missing, an unpinned brick is missing only
	// when no version is installed, and a pinned one when its version is not installed. The
	// namespace does not matter: a brick that does not ship with the cli is installed.
	require.Equal(t, []app.Brick{
		{ID: "arduino:published"},
		{ID: "acme:camera", Version: "2.0.0"},
		{ID: "acme:audio"},
	}, missingBricks(a, a.Bricks(index)))
}
