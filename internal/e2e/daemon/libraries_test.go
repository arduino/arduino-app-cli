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
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/e2e/client"
)

func TestListLibraries(t *testing.T) {
	httpClient := GetHttpclient(t)

	createResp, err := httpClient.ListLibrariesWithResponse(
		t.Context(),
		&client.ListLibrariesParams{},
	)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, createResp.StatusCode())
	require.NotNil(t, createResp.JSON200, "The creation response body should not be nil")
	require.True(t, len(*createResp.JSON200.Libraries) > 0, "The created app ID should not be nil")
}

func TestListLibrariesWithParams(t *testing.T) {
	httpClient := GetHttpclient(t)

	createResp, err := httpClient.ListLibrariesWithResponse(
		t.Context(),
		&client.ListLibrariesParams{
			Search: f.Ptr("Modulino"),
			Limit:  f.Ptr(1),
		},
	)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, createResp.StatusCode())
	require.NotNil(t, createResp.JSON200, "The creation response body should not be nil")
	require.True(t, len(*createResp.JSON200.Libraries) == 1, "There must at least one Modulino library matching the search term (we hope so...)")
	require.Equal(t, f.Ptr("https://github.com/arduino-libraries/Modulino"), (*createResp.JSON200.Libraries)[0].Website, "The website must match the search term")
}
