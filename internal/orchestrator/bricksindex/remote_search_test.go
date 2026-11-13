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
	release := func(version string, boards ...string) RemoteRelease {
		return RemoteRelease{Version: semver.MustParse(version), SupportedBoards: boards}
	}
	index := &RemoteIndex{Bricks: []RemoteBrick{
		{
			ID: "sensorlab:air_quality", Name: "Air Quality Monitor", Namespace: "sensorlab", Author: "SensorLab",
			Description: "Read air quality metrics from I2C sensors.",
			Releases:    []RemoteRelease{release("0.3.1")},
		},
		{
			ID: "acme:camera", Name: "Camera Brick", Namespace: "acme", Author: "ACME Corporation",
			Description: "Capture frames from USB cameras.",
			Releases:    []RemoteRelease{release("1.0.0"), release("1.1.0"), release("2.0.0", "ventunoq")},
		},
		{
			ID: "acme:audio", Name: "Audio", Namespace: "acme", Author: "ACME Corporation",
			Releases: []RemoteRelease{release("1.0.0", "ventunoq")},
		},
	}}

	ids := func(bricks []*RemoteBrick) []string {
		res := make([]string, 0, len(bricks))
		for _, b := range bricks {
			res = append(res, b.ID)
		}
		return res
	}

	unoq := platform.Platform{BoardName: "unoq"}
	tests := []struct {
		query    string
		platform platform.Platform
		want     []string
	}{
		{"", unoq, []string{"acme:camera", "sensorlab:air_quality"}},
		{"", platform.Platform{BoardName: "ventunoq"}, []string{"acme:audio", "acme:camera", "sensorlab:air_quality"}},
		{"", platform.Platform{}, []string{"acme:audio", "acme:camera", "sensorlab:air_quality"}},
		{"CAMERA", unoq, []string{"acme:camera"}},
		{"usb frames", unoq, []string{"acme:camera"}},
		{"usb sensors", unoq, []string{}},
		{"corporation", unoq, []string{"acme:camera"}},
		{"i2c", unoq, []string{"sensorlab:air_quality"}},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q board=%q", tt.query, tt.platform.BoardName), func(t *testing.T) {
			require.Equal(t, tt.want, ids(index.Search(tt.query, tt.platform)))
		})
	}
}
