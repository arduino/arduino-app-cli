// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package apprelease

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReleaseDefaultFileName(t *testing.T) {
	const num = "20260709120000"

	cases := []struct {
		name string
		app  string
		want string
	}{
		{"spaces lowercased", "My Weather Station", "my-weather_" + num + ".tar.gz"},
		{"path separators stripped", "sensors/v2", "sensors-v2_" + num + ".tar.gz"},
		{"backslash and dots stripped", `a.b\c`, "a-b-c_" + num + ".tar.gz"},
		{"punctuation dropped and truncated", "Robot#1 (beta)!", "robot1-bet_" + num + ".tar.gz"},
		{"truncated to 10 chars", "abcdefghijklmnop", "abcdefghij_" + num + ".tar.gz"},
		{"non-ascii dropped, no split rune", "日本語app", "app_" + num + ".tar.gz"},
		{"empty falls back to app", "///", "app_" + num + ".tar.gz"},
		{"trailing dashes trimmed", "hello---", "hello_" + num + ".tar.gz"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := releaseDefaultFileName(tc.app, num)
			require.Equal(t, tc.want, got)
			// Never a nested path, always the expected extension.
			require.NotContains(t, got, "/")
			require.NotContains(t, got, `\`)
			require.True(t, strings.HasSuffix(got, ".tar.gz"))
		})
	}
}
