// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package daemon

import (
	"cmp"
	"context"
	"net/http"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/e2e"
	"github.com/arduino/arduino-app-cli/internal/e2e/client"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
)

func TestModelHandlerDownloadFlow(t *testing.T) {
	if runtime.GOARCH != "arm64" {
		t.Skipf("Skipping test: requires arm64 architecture, currently running on %s", runtime.GOARCH)
	}
	modelID := cmp.Or(os.Getenv("E2E_MODEL_ID"), "melo-tts-es")
	// The API takes the encoded form; modelID stays plain for the messages below.
	encodedID := modelsindex.EncodeID(modelID)

	modelsDir := e2e.FindRepositoryRootPath(t).Join("models")
	t.Cleanup(func() { _ = modelsDir.RemoveAll() })

	httpClient, daemonAddr := GetHttpclientAndAddr(t, e2e.WithModelsDir(modelsDir), e2e.WithBoardName("ventunoq"))
	requestEditor := func(_ context.Context, _ *http.Request) error { return nil }
	time.Sleep(2 * time.Second)

	t.Run("model is not installed before download", func(t *testing.T) {
		resp, err := getModelWithRetry(t, httpClient, encodedID, requestEditor)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), "model %q not found in index", modelID)
		require.NotNil(t, resp.JSON200)
		require.Equal(t, client.ModelStatus("not-installed"), resp.JSON200.Status, "model should not be installed in a fresh environment")
	})

	t.Run("install emits progress events", func(t *testing.T) {

		req, err := http.NewRequest(http.MethodPut, daemonAddr+"/v1/models/"+encodedID, nil) //nolint:gosec
		assert.NoError(t, err, "failed to create request for model install")
		events, err := newSSEClient(req, 0)
		require.NoError(t, err)
		hasProgress := false
		hasComplete := false
		for e := range events {
			t.Log("Received SSE event", "id", e.ID, "event", e.Event, "data", string(e.Data))
			if e.Event == "progress" {
				hasProgress = true
			}
			if e.Event == "done" {
				hasComplete = true
			}
		}

		require.True(t, hasProgress, "expected at least one 'progress' SSE event")
		require.True(t, hasComplete, "expected at least one 'complete' SSE event")

	})

	t.Run("model is installed after download", func(t *testing.T) {
		resp, err := getModelWithRetry(t, httpClient, encodedID, requestEditor)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		require.Equal(t, client.ModelStatus("installed"), resp.JSON200.Status, "model should be installed after successful download")
	})

	t.Run("model can be deleted", func(t *testing.T) {
		force := false
		resp, err := httpClient.DeleteAIModelWithResponse(
			t.Context(), encodedID,
			&client.DeleteAIModelParams{Force: &force},
			requestEditor,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, resp.StatusCode(),
			"DELETE should return 204 without errors")
	})

	t.Run("model is not installed after delete", func(t *testing.T) {
		resp, err := getModelWithRetry(t, httpClient, encodedID, requestEditor)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		require.Equal(t, client.ModelStatus("not-installed"), resp.JSON200.Status, "model should not be installed after delete")
	})
}

// getModelWithRetry retries GetAIModelDetailsWithResponse up to 5 times with a 1s
// delay between attempts to tolerate transient Docker errors.
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
