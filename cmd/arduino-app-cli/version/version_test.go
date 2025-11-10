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

package version

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetValidUrl(t *testing.T) {
	testCases := []struct {
		name           string
		hostPort       string
		expectedResult string
	}{
		{
			name:           "Valid host and port should return default.",
			hostPort:       "localhost:8800",
			expectedResult: "localhost:8800",
		},
		{
			name:           "Missing host should return default host.",
			hostPort:       ":8800",
			expectedResult: "localhost:8800",
		},
		{
			name:           "Missing port should return default port.",
			hostPort:       "localhost:",
			expectedResult: "localhost:8800",
		},
		{
			name:           "Custom host and port should return the provided host:port.",
			hostPort:       "192.168.100.1:1234",
			expectedResult: "192.168.100.1:1234",
		},
		{
			name:           "Host only should return provided input and default port.",
			hostPort:       "192.168.1.1",
			expectedResult: "192.168.1.1:8800",
		},
		{
			name:           "Missing host and port should return default.",
			hostPort:       "",
			expectedResult: "localhost:8800",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url, _ := validateHost(tc.hostPort)
			require.Equal(t, tc.expectedResult, url)
		})
	}
}

func TestServerVersion(t *testing.T) {
	clientVersion := "5.1-dev"
	unreacheableUrl := "unreacheable:123"
	daemonVersion := ""

	testCases := []struct {
		name           string
		serverStub     Tripper
		expectedResult versionResult
		hostAndPort    string
	}{
		{
			name:       "return the server version when the server is up",
			serverStub: successServer,
			expectedResult: versionResult{
				Name:          ProgramName,
				Version:       clientVersion,
				DaemonVersion: "3.0",
			},
			hostAndPort: "localhost:8800",
		},
		{
			name:       "return error if default server is not listening",
			serverStub: failureServer,
			expectedResult: versionResult{
				Name:          ProgramName,
				Version:       clientVersion,
				DaemonVersion: daemonVersion,
			},
			hostAndPort: unreacheableUrl,
		},
		{
			name:       "return error if provided server is not listening",
			serverStub: failureServer,
			expectedResult: versionResult{
				Name:          ProgramName,
				Version:       clientVersion,
				DaemonVersion: daemonVersion,
			},
			hostAndPort: unreacheableUrl,
		},
		{
			name:       "return error for server resopnse 500 Internal Server Error",
			serverStub: failureInternalServerError,
			expectedResult: versionResult{
				Name:          ProgramName,
				Version:       clientVersion,
				DaemonVersion: daemonVersion,
			},
			hostAndPort: unreacheableUrl,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// arrange
			httpClient := http.Client{}
			httpClient.Transport = tc.serverStub

			// act
			result, _ := versionHandler(httpClient, clientVersion, tc.hostAndPort)

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
