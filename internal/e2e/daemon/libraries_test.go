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
