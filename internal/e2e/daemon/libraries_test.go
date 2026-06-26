// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package daemon

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

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
			Search: new("Modulino"),
			Limit:  new(1),
		},
	)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, createResp.StatusCode())
	require.NotNil(t, createResp.JSON200, "The creation response body should not be nil")
	require.True(t, len(*createResp.JSON200.Libraries) == 1, "There must at least one Modulino library matching the search term (we hope so...)")
	require.Equal(t, new("https://github.com/arduino-libraries/Arduino_Modulino"), (*createResp.JSON200.Libraries)[0].Website, "The website must match the search term")
}
