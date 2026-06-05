// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package daemon

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/e2e"
	"github.com/arduino/arduino-app-cli/internal/e2e/client"
)

func GetHttpclient(t *testing.T, opts ...e2e.ArduinoAppCLIOption) *client.ClientWithResponses {
	t.Helper()
	c, _ := GetHttpclientAndAddr(t, opts...)
	return c
}

// GetHttpclientAndAddr returns the HTTP client together with the daemon base URL.
// Use this when you need to make raw HTTP requests (e.g. SSE streams).
func GetHttpclientAndAddr(t *testing.T, opts ...e2e.ArduinoAppCLIOption) (*client.ClientWithResponses, string) {
	t.Helper()
	cli := e2e.CreateEnvForDaemon(t, opts...)
	t.Cleanup(cli.CleanUp)
	httpClient, err := client.NewClientWithResponses(cli.DaemonAddr)
	require.NoError(t, err)
	return httpClient, cli.DaemonAddr
}
