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

package daemon

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCors(t *testing.T) {
	httpClient := GetHttpclient(t)

	tests := []struct {
		origin      string
		shouldAllow bool
	}{
		{"wails://wails", true},
		{"wails://wails.localhost", true},
		{"wails://wails.localhost:8000", true},

		{"http://wails.localhost", true},
		{"http://wails.localhost:8001", true},

		{"http://localhost", true},
		{"http://localhost:8002", true},
		{"https://localhost", true},

		// not valid, should not be allowed
		{"http://randomsite.com", false},
	}

	for _, tc := range tests {
		t.Run(tc.origin, func(t *testing.T) {
			addHeaders := func(ctx context.Context, req *http.Request) error {
				req.Header.Set("origin", tc.origin)
				return nil
			}
			resp, err := httpClient.GetVersions(t.Context(), addHeaders)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, 200, resp.StatusCode)
			if tc.shouldAllow {
				require.Equal(t, tc.origin, resp.Header.Get("Access-Control-Allow-Origin"))
			} else {
				require.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))
			}
		})
	}
}
