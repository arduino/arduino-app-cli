// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckOrigin(t *testing.T) {
	origins := []string{
		"wails://wails",
		"wails://wails.localhost:*",
		"http://wails.localhost:*",
		"http://localhost:*",
		"https://localhost:*",
		"http://example.com:7000",
		"https://*:443",
	}

	allow := func(origin string) {
		require.True(t, checkOrigin(origin, origins), "Expected origin %s to be allowed", origin)
	}
	deny := func(origin string) {
		require.False(t, checkOrigin(origin, origins), "Expected origin %s to be denied", origin)
	}
	allow("wails://wails")
	allow("wails://wails:8000")
	allow("http://wails.localhost")
	allow("http://localhost")
	allow("http://example.com:7000")
	allow("https://blah.com:443")
	deny("wails://evil.com")
	deny("https://wails.localhost:8000")
	deny("http://example.com:8000")
	deny("http://blah.com:443")
	deny("https://blah.com:8080")
}
