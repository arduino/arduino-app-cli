// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricksindex

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/platform"
)

func TestSearch(t *testing.T) {
	index := &RemoteIndex{Bricks: []RemoteBrick{
		{
			ID: "sensorlab:air_quality", Name: "Air Quality Monitor", Namespace: "sensorlab", Version: semver.MustParse("0.3.1"),
			Description: "Read air quality metrics from I2C sensors.", Deprecated: true,
		},
		{
			ID: "acme:camera", Name: "Camera Brick", Namespace: "acme", Version: semver.MustParse("1.0.0"),
			Description: "Old description mentioning thermal.",
		},
		{
			ID: "acme:camera", Name: "Camera Brick", Namespace: "acme", Version: semver.MustParse("1.1.0"),
			Description: "Capture frames from USB cameras.", Repository: "https://gitlab.com/acme/camera-brick",
		},
		{
			ID: "acme:camera", Name: "Camera Brick", Namespace: "acme", Version: semver.MustParse("2.0.0"),
			Description: "Capture frames from MIPI cameras.", SupportedBoards: []string{"ventunoq"},
		},
		{
			ID: "acme:audio", Name: "Audio", Namespace: "acme", Version: semver.MustParse("1.0.0"),
			SupportedBoards: []string{"ventunoq"},
		},
	}}

	releases := func(bricks []*RemoteBrick) []string {
		res := make([]string, 0, len(bricks))
		for _, b := range bricks {
			res = append(res, b.ID+"@"+b.Version.String())
		}
		return res
	}

	unoq := platform.Platform{BoardName: "unoq"}
	tests := []struct {
		query             string
		includeDeprecated bool
		platform          platform.Platform
		want              []string
	}{
		{"", false, unoq, []string{"acme:camera@1.1.0"}},
		{"", true, unoq, []string{"acme:camera@1.1.0", "sensorlab:air_quality@0.3.1"}},
		{"", false, platform.Platform{BoardName: "ventunoq"}, []string{"acme:audio@1.0.0", "acme:camera@2.0.0"}},
		{"", false, platform.Platform{}, []string{"acme:audio@1.0.0", "acme:camera@2.0.0"}},
		{"CAMERA", false, unoq, []string{"acme:camera@1.1.0"}},
		{"usb frames", false, unoq, []string{"acme:camera@1.1.0"}},
		{"usb sensors", true, unoq, []string{}},
		{"gitlab", false, unoq, []string{"acme:camera@1.1.0"}},
		{"thermal", false, unoq, []string{}},
		{"mipi", false, unoq, []string{}},
		{"i2c", false, unoq, []string{}},
		{"i2c", true, unoq, []string{"sensorlab:air_quality@0.3.1"}},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q deprecated=%t board=%q", tt.query, tt.includeDeprecated, tt.platform.BoardName), func(t *testing.T) {
			require.Equal(t, tt.want, releases(index.Search(tt.query, tt.includeDeprecated, tt.platform)))
		})
	}
}
