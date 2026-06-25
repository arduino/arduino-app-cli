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

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/e2e"
	"github.com/arduino/arduino-app-cli/internal/e2e/client"
)

// TestModelHandlerDownloadFlow exercises the full install → verify → delete cycle
// for a handler-backed model, validating that:
//   - Docker containers are spawned correctly via the Docker API
//   - SSE progress events are emitted and parsed
//   - The model is reported as installed after download
//   - The delete container runs without errors
//
// Optional E2E_MODEL_ID env var for changing the model ID to test. Defaults to "melo-tts-es".
func TestModelHandlerDownloadFlow(t *testing.T) {
	modelID := cmp.Or(os.Getenv("E2E_MODEL_ID"), "melo-tts-es")

	modelsDir := paths.New(t.TempDir()).Join("custom-models")
	httpClient, daemonAddr := GetHttpclientAndAddr(t, e2e.WithCustomModelDir(modelsDir), e2e.WithBoardName("ventunoq"))
	requestEditor := func(_ context.Context, _ *http.Request) error { return nil }

	t.Run("model is not installed before download", func(t *testing.T) {
		resp, err := getModelWithRetry(t, httpClient, modelID, requestEditor)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), "model %q not found in index", modelID)
		require.NotNil(t, resp.JSON200)
		require.NotNil(t, resp.JSON200.Installed)
		require.False(t, *resp.JSON200.Installed, "model should not be installed in a fresh environment")
	})

	t.Run("install emits progress events", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute)
		defer cancel()

		events, err := collectInstallSSE(ctx, daemonAddr, modelID)
		require.NoError(t, err)
		require.NotEmpty(t, events, "expected at least one SSE event from the install stream")

		bySSEType := make(map[string][]handlerSSEEvent)
		for _, e := range events {
			bySSEType[e.SSEType] = append(bySSEType[e.SSEType], e)
		}

		require.Contains(t, bySSEType, "progress", "expected at least one 'progress' SSE event")
		firstProgress := bySSEType["progress"][0]
		require.Greater(t, firstProgress.Total, int64(0), "'progress' event must carry the file size as Total")
	})

	t.Run("model is installed after download", func(t *testing.T) {
		resp, err := getModelWithRetry(t, httpClient, modelID, requestEditor)
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
			"DELETE should return 204 without errors")
	})

	t.Run("model is not installed after delete", func(t *testing.T) {
		resp, err := getModelWithRetry(t, httpClient, modelID, requestEditor)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		require.False(t, *resp.JSON200.Installed, "model should not be installed after delete")
	})
}

// handlerSSEEvent mirrors the SSE events emitted by the daemon's install handler.
// SSEType is the value from the SSE "event:" line; the remaining fields cover both
// the "message" payload {"message":...} and the "progress" payload {"name":...,"total":...}.
type handlerSSEEvent struct {
	SSEType  string  `json:"-"`
	Name     string  `json:"name"`
	Total    int64   `json:"total"`
	Current  int64   `json:"current"`
	Progress float32 `json:"progress"`
	Message  string  `json:"message"`
	Code     string  `json:"code"`
}

// collectInstallSSE issues PUT /v1/models/{id} and collects SSE events until the
// server sends an "close" event (stream ended) or the context is canceled.
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
	var currentSSEType string

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			currentSSEType = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			dataBuf.WriteString(strings.TrimPrefix(line, "data: "))
		case line == "" && dataBuf.Len() > 0:
			var e handlerSSEEvent
			if err := json.Unmarshal([]byte(dataBuf.String()), &e); err == nil {
				e.SSEType = currentSSEType
				events = append(events, e)
				if e.SSEType == "close" {
					return events, nil
				}
			}
			dataBuf.Reset()
			currentSSEType = ""
		}
	}
	return events, scanner.Err()
}

// getModelWithRetry retries GetAIModelDetailsWithResponse up to 5 times with a 1s
// delay between attempts to tolerate the Docker check-container output race with AutoRemove.
func getModelWithRetry(t *testing.T, httpClient *client.ClientWithResponses, modelID string, editor client.RequestEditorFn) (*client.GetAIModelDetailsResp, error) {
	t.Helper()
	const (
		maxAttempts = 5
		retryDelay  = time.Second
	)
	var (
		resp *client.GetAIModelDetailsResp
		err  error
	)
	for range maxAttempts {
		resp, err = httpClient.GetAIModelDetailsWithResponse(t.Context(), modelID, editor)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode() != http.StatusInternalServerError {
			return resp, nil
		}
		time.Sleep(retryDelay)
	}
	return resp, nil
}
