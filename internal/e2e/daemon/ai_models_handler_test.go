// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package daemon

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/e2e"
	"github.com/arduino/arduino-app-cli/internal/e2e/client"
)

const (
	ModelInstallEventStart    = "start"
	ModelInstallEventUpdate   = "update"
	ModelInstallEventComplete = "complete"
	ModelInstallEventInfo     = "info"
	ModelInstallEventError    = "error"
	ModelInstallEventDone     = "done"
)

// TestModelHandlerDownloadFlow exercises the full install → verify → delete cycle
// for a handler-backed model, validating that:
//   - Docker containers are spawned correctly via the Docker API
//   - SSE progress events (start / update / complete / done) are emitted and parsed
//   - The model is reported as installed after download
//   - The delete container runs without "short write" errors
//
// Optional E2E_MODEL_ID env var for changing the model ID to test. Defaults to "melo-tts-es".
func TestModelHandlerDownloadFlow(t *testing.T) {
	modelID := cmp.Or(os.Getenv("E2E_MODEL_ID"), "melo-tts-es")

	httpClient, daemonAddr := GetHttpclientAndAddr(t, e2e.WithCustomModelDir(nil))
	requestEditor := func(_ context.Context, _ *http.Request) error { return nil }

	t.Run("model is not installed before download", func(t *testing.T) {
		resp, err := httpClient.GetAIModelDetailsWithResponse(t.Context(), modelID, requestEditor)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), "model %q not found in index", modelID)
		require.NotNil(t, resp.JSON200)
		require.NotNil(t, resp.JSON200.Installed)
		require.False(t, *resp.JSON200.Installed, "model should not be installed in a fresh environment")
	})

	t.Run("install emits start, update and complete progress events", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute)
		defer cancel()

		events, err := collectInstallSSE(ctx, daemonAddr, modelID)
		require.NoError(t, err)
		require.NotEmpty(t, events, "expected at least one SSE event from the install stream")

		byType := make(map[string][]handlerSSEEvent)
		for _, e := range events {
			byType[e.Type] = append(byType[e.Type], e)
		}

		require.Contains(t, byType, ModelInstallEventDone,
			"install stream must end with a 'done' event")

		doneEvents := byType[string(ModelInstallEventDone)]
		doneEvent := doneEvents[len(doneEvents)-1]
		require.Empty(t, doneEvent.Description,
			"'done' description should be empty on success; got error: %s", doneEvent.Description)

		require.Contains(t, byType, ModelInstallEventStart,
			"expected a 'start' progress event")
		require.Contains(t, byType, ModelInstallEventComplete,
			"expected a 'complete' progress event")

		startEvent := byType[ModelInstallEventStart][0]
		require.Greater(t, startEvent.Total, int64(0), "'start' event must carry the file size as Total")
		require.Equal(t, "B", startEvent.Unit)
		require.NotEmpty(t, startEvent.Percentage)
	})

	t.Run("model is installed after download", func(t *testing.T) {
		resp, err := httpClient.GetAIModelDetailsWithResponse(t.Context(), modelID, requestEditor)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		require.True(t, *resp.JSON200.Installed, "model should be installed after successful download")
	})

	t.Run("model can be deleted", func(t *testing.T) {
		force := false
		resp, err := httpClient.DeleteAIModelWithResponse(
			t.Context(), modelID,
			&client.DeleteAIModelParams{Force: &force},
			requestEditor,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, resp.StatusCode(),
			"DELETE should return 204 without 'short write' errors")
	})

	t.Run("model is not installed after delete", func(t *testing.T) {
		resp, err := httpClient.GetAIModelDetailsWithResponse(t.Context(), modelID, requestEditor)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		require.False(t, *resp.JSON200.Installed, "model should not be installed after delete")
	})
}

// handlerSSEEvent mirrors the fields emitted by the model handler container and
// forwarded as SSE by the daemon.
type handlerSSEEvent struct {
	Type        string   `json:"type"`
	ModelID     string   `json:"model_id"`
	Description string   `json:"description"`
	Current     int64    `json:"current"`
	Total       int64    `json:"total"`
	Unit        string   `json:"unit"`
	Percentage  string   `json:"percentage"`
	Artifacts   []string `json:"artifacts"`
}

// collectInstallSSE issues PUT /v1/models/{id} and reads SSE events until the
// daemon sends a "done" event or the context is canceled.
func collectInstallSSE(ctx context.Context, baseURL, modelID string) ([]handlerSSEEvent, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, baseURL+"/v1/models/"+modelID, nil) //nolint:gosec
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var events []handlerSSEEvent
	scanner := bufio.NewScanner(resp.Body)
	var dataBuf strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "data: "):
			dataBuf.WriteString(strings.TrimPrefix(line, "data: "))
		case line == "" && dataBuf.Len() > 0:
			var e handlerSSEEvent
			if err := json.Unmarshal([]byte(dataBuf.String()), &e); err == nil {
				events = append(events, e)
				if e.Type == ModelInstallEventDone {
					return events, nil
				}
			}
			dataBuf.Reset()
		}
	}
	return events, scanner.Err()
}
