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
