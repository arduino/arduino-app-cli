// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package remotebricksindex

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSearch(t *testing.T) {
	index := &Index{Bricks: []Brick{
		{
			ID: "acme:camera", Name: "Camera Brick", Namespace: "acme", Latest: "1.1.0",
			Releases: []Release{
				{Version: "1.1.0", Description: "Capture frames from USB cameras.", Repository: "https://gitlab.com/acme/camera-brick"},
				{Version: "1.0.0", Description: "Old description mentioning thermal."},
			},
		},
		{
			ID: "sensorlab:air_quality", Name: "Air Quality Monitor", Namespace: "sensorlab", Latest: "0.3.1", Deprecated: true,
			Releases: []Release{{Version: "0.3.1", Description: "Read air quality metrics from I2C sensors."}},
		},
	}}

	ids := func(bricks []Brick) []string {
		res := make([]string, 0, len(bricks))
		for _, b := range bricks {
			res = append(res, b.ID)
		}
		return res
	}

	tests := []struct {
		query             string
		includeDeprecated bool
		want              []string
	}{
		{"", false, []string{"acme:camera"}},
		{"", true, []string{"acme:camera", "sensorlab:air_quality"}},
		{"CAMERA", false, []string{"acme:camera"}},
		{"usb frames", false, []string{"acme:camera"}},
		{"usb sensors", true, []string{}},
		{"gitlab", false, []string{"acme:camera"}},
		{"thermal", false, []string{}},
		{"i2c", false, []string{}},
		{"i2c", true, []string{"sensorlab:air_quality"}},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q deprecated=%t", tt.query, tt.includeDeprecated), func(t *testing.T) {
			require.Equal(t, tt.want, ids(index.Search(tt.query, tt.includeDeprecated)))
		})
	}
}
