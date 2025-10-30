// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

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
	allow("http://example.com:7000")
	allow("https://blah.com:443")
	deny("wails://evil.com")
	deny("https://wails.localhost:8000")
	deny("http://example.com:8000")
	deny("http://blah.com:443")
	deny("https://blah.com:8080")
}
