// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
)

func TestBrickImages(t *testing.T) {
	const image = "ghcr.io/acme/camera:1.0@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	composeFile := paths.New(t.TempDir(), "brick_compose.yaml")
	require.NoError(t, composeFile.WriteFile([]byte("services:\n  camera:\n    image: "+image+"\n")))
	services := &servicesindex.ServicesIndex{
		Services: []servicesindex.Service{
			{ServiceID: "arduino:genie", ComposeFile: paths.New("testdata", "composes", "service_compose_genie.yaml")},
		},
	}

	// The images that are not ours are included: a third-party brick runs its own.
	images, err := BrickImages(bricksindex.Brick{
		ID:               "acme:camera",
		ComposeFile:      composeFile,
		RequiresServices: bricksindex.RequiresServices{{ID: "arduino:genie"}, {ID: "arduino:missing"}},
	}, services)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{image, "ghcr.io/arduino/app-bricks/genie-service:1.0.0"}, images)

	images, err = BrickImages(bricksindex.Brick{ID: "acme:empty"}, services)
	require.NoError(t, err)
	require.Empty(t, images)
}
