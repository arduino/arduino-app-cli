// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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
