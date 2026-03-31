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
