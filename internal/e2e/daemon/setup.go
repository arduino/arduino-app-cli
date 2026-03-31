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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/e2e"
	"github.com/arduino/arduino-app-cli/internal/e2e/client"
)

func GetHttpclient(t *testing.T, opts ...e2e.ArduinoAppCLIOption) *client.ClientWithResponses {
	t.Helper()
	cli := e2e.CreateEnvForDaemon(t, opts...)
	t.Cleanup(cli.CleanUp)
	httpClient, err := client.NewClientWithResponses(cli.DaemonAddr)
	require.NoError(t, err)

	return httpClient
}
