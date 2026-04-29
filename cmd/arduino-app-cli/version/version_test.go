// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package version

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDaemonVersion(t *testing.T) {
	testCases := []struct {
		name                 string
		serverStub           Tripper
		port                 string
		expectedResult       string
		expectedErrorMessage string
	}{
		{
			name:                 "return the server version when the server is up",
			serverStub:           successServer,
			port:                 "8800",
			expectedResult:       "3.0-server",
			expectedErrorMessage: "",
		},
		{
			name:                 "return error if default server is not listening on default port",
			serverStub:           failureServer,
			port:                 "8800",
			expectedResult:       "",
			expectedErrorMessage: `Get "http://localhost:8800/v1/version": connection refused`,
		},
		{
			name:                 "return error if provided server is not listening on provided port",
			serverStub:           failureServer,
			port:                 "1234",
			expectedResult:       "",
			expectedErrorMessage: `Get "http://localhost:1234/v1/version": connection refused`,
		},
		{
			name:                 "return error for server response 500 Internal Server Error",
			serverStub:           failureInternalServerError,
			port:                 "0",
			expectedResult:       "",
			expectedErrorMessage: "unexpected status code received",
		},

		{
			name:                 "return error for server up and wrong json response",
			serverStub:           successServerWrongJson,
			port:                 "8800",
			expectedResult:       "",
			expectedErrorMessage: "invalid character '<' looking for beginning of value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// arrange
			httpClient := http.Client{}
			httpClient.Transport = tc.serverStub

			// act
			result, err := getDaemonVersion(httpClient, tc.port)

			// assert
			require.Equal(t, tc.expectedResult, result)
			if err != nil {
				require.Equal(t, tc.expectedErrorMessage, err.Error())
			}
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
	body := io.NopCloser(strings.NewReader(`{"version":"3.0-server"}`))
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	}, nil
})

var successServerWrongJson = Tripper(func(*http.Request) (*http.Response, error) {
	body := io.NopCloser(strings.NewReader(`<!doctype html><html lang="en"`))
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	}, nil
})

var failureServer = Tripper(func(*http.Request) (*http.Response, error) {
	return nil, errors.New("connection refused")
})

var failureInternalServerError = Tripper(func(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader("")),
	}, nil
})
