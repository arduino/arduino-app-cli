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
}
