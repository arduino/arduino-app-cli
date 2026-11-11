// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	appmodel "github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

func TestNewAppDetailsResult(t *testing.T) {
	arduinoApp := appmodel.ArduinoApp{
		Descriptor: appmodel.AppDescriptor{Bricks: []appmodel.Brick{{
			ID:        "arduino:cloud",
			Model:     "cloud-model",
			Variables: map[string]string{"OPTIONAL": "configured"},
		}}},
	}
	bricksIndex := &bricksindex.BricksIndex{BuiltInBricks: []bricksindex.Brick{{
		ID:       "arduino:cloud",
		Name:     "Cloud",
		Category: "network",
		Variables: []bricksindex.BrickVariable{
			{Name: "REQUIRED", Description: "Required variable"},
			{Name: "OPTIONAL", Description: "Optional variable", DefaultValue: "default"},
			{Name: "HIDDEN", Hidden: true},
		},
	}}}
	result := newAppDetailsResult(orchestrator.AppDetailedInfo{
		Name:        "Weather",
		Path:        "/apps/weather",
		Description: "Weather station",
		Icon:        "☀️",
		Status:      orchestrator.StatusStopped,
		Default:     true,
	}, arduinoApp, bricksIndex)

	require.Equal(t, appDetailsResult{
		ID:          "",
		Name:        "Weather",
		Path:        "/apps/weather",
		Description: "Weather station",
		Icon:        "☀️",
		Status:      orchestrator.StatusStopped,
		Default:     true,
		Bricks: []appDetailsBrick{{
			ID:       "arduino:cloud",
			Name:     "Cloud",
			Category: "network",
			Model:    "cloud-model",
			Variables: []appDetailsVariable{
				{Name: "REQUIRED", Description: "Required variable", Required: true},
				{Name: "OPTIONAL", Description: "Optional variable", Configured: true},
			},
		}},
	}, result)
	require.Equal(t, `ID: 
Name: Weather
Description: Weather station
Icon: ☀️
Path: /apps/weather
Status: stopped
Example: false
Default: true

Bricks:
  arduino:cloud (Cloud)
    Category: network
    Model: cloud-model
    Variables:
      REQUIRED (required, not configured): Required variable
      OPTIONAL (optional, configured): Optional variable`, result.String())
}
