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
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tmaxmax/go-sse"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

func TestSystemResources(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("System resources test is only applicable for Linux")
	}

	httpClient := GetHttpclient(t)
	t.Run("GetResources_Success_Receives_SSE_Events", func(t *testing.T) {
		//nolint:bodyclose
		systemResources, err := httpClient.GetSystemResources(t.Context())
		require.NoError(t, err)

		reqCtx, cancelCtx := context.WithTimeout(t.Context(), 1*time.Minute)
		conn := sse.DefaultClient.NewConnection(systemResources.Request.WithContext(reqCtx))

		var (
			cpuResp  orchestrator.SystemCPUResource
			memResp  orchestrator.SystemMemoryResource
			diskResp orchestrator.SystemDiskResource
		)

		conn.SubscribeToAll(func(event sse.Event) {
			switch event.Type {
			case "cpu":
				require.NoError(t, json.Unmarshal([]byte(event.Data), &cpuResp))
			case "mem":
				require.NoError(t, json.Unmarshal([]byte(event.Data), &memResp))
			case "disk":
				require.NoError(t, json.Unmarshal([]byte(event.Data), &diskResp))
			}
			if cpuResp != (orchestrator.SystemCPUResource{}) &&
				memResp != (orchestrator.SystemMemoryResource{}) &&
				diskResp != (orchestrator.SystemDiskResource{}) {
				cancelCtx() // Stop the connection once we have all resources
			}
		})

		err = conn.Connect()
		if !errors.Is(err, context.Canceled) {
			require.NoError(t, err)
		}
		require.NotEmpty(t, cpuResp.UsedPercent)
		require.NotEmpty(t, memResp.Used)
		require.NotEmpty(t, memResp.Total)
		require.NotEmpty(t, diskResp.Path)
		require.NotEmpty(t, diskResp.Used)
		require.NotEmpty(t, diskResp.Total)
	})
}
