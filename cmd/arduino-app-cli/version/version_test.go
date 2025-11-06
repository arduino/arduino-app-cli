package version

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServerVersion(t *testing.T) {
	clientVersion := "5.1-dev"

	testCases := []struct {
		name           string
		serverStub     Tripper
		expectedResult versionResult
		host           string
	}{
		{
			name:       "return the server version when the server is up",
			serverStub: successServer,
			expectedResult: versionResult{
				ClientVersion: "5.1-dev",
				ServerVersion: "3.0",
			},
			host: "",
		},
		{
			name:       "return error if default server is not listening",
			serverStub: failureServer,
			expectedResult: versionResult{
				ClientVersion: "5.1-dev",
				ServerVersion: fmt.Sprintf("n/a (cannot connect to the server http://%s:%s)", DefaultHostname, DefaultPort),
			},
			host: "",
		},
		{
			name:       "return error if provided server is not listening",
			serverStub: failureServer,
			expectedResult: versionResult{
				ClientVersion: "5.1-dev",
				ServerVersion: "n/a (cannot connect to the server http://unreacheable:123)",
			},
			host: "unreacheable:123",
		},
		{
			name:       "return error for server resopnse 500 Internal Server Error",
			serverStub: failureInternalServerError,
			expectedResult: versionResult{
				ClientVersion: "5.1-dev",
				ServerVersion: "n/a (cannot connect to the server http://unreacheable:123)",
			},
			host: "unreacheable:123",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// arrange
			httpClient := http.Client{}
			httpClient.Transport = tc.serverStub

			// act
			result := doVersionHandler(httpClient, clientVersion, tc.host)

			// assert
			require.Equal(t, tc.expectedResult, result)
		})
	}
}

// Leverage the http.Client's RoundTripper
// to return a canned response and bypass network calls.
type Tripper func(*http.Request) (*http.Response, error)

func (t Tripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return t(request)
}

var successServer = Tripper(func(*http.Request) (*http.Response, error) {
	body := io.NopCloser(strings.NewReader(`{"version":"3.0"}`))
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	}, nil
})

var failureServer = Tripper(func(*http.Request) (*http.Response, error) {
	return nil, errors.New("connetion refused")
})

var failureInternalServerError = Tripper(func(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader("")),
	}, nil
})
