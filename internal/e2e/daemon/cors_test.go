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
