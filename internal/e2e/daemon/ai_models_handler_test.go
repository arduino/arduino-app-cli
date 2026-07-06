// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package daemon

import (
	"bufio"
	"cmp"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/e2e"
	"github.com/arduino/arduino-app-cli/internal/e2e/client"
)

func TestModelHandlerDownloadFlow(t *testing.T) {
	if runtime.GOARCH != "arm64" {
		t.Skipf("Skipping test: requires arm64 architecture, currently running on %s", runtime.GOARCH)
	}
	modelID := cmp.Or(os.Getenv("E2E_MODEL_ID"), "melo-tts-es")

	modelsDir := e2e.FindRepositoryRootPath(t).Join("models")
	t.Cleanup(func() { _ = modelsDir.RemoveAll() })

	httpClient, daemonAddr := GetHttpclientAndAddr(t, e2e.WithModelsDir(modelsDir), e2e.WithBoardName("ventunoq"))
	requestEditor := func(_ context.Context, _ *http.Request) error { return nil }
	time.Sleep(2 * time.Second)

	t.Run("model is not installed before download", func(t *testing.T) {
		resp, err := getModelWithRetry(t, httpClient, modelID, requestEditor)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), "model %q not found in index", modelID)
		require.NotNil(t, resp.JSON200)
		require.NotNil(t, resp.JSON200.Status)
		require.Equal(t, "not-installed", *resp.JSON200.Status, "model should not be installed in a fresh environment")
	})

	t.Run("install emits progress events", func(t *testing.T) {

		req, err := http.NewRequest(http.MethodPut, daemonAddr+"/v1/models/"+modelID, nil) //nolint:gosec
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
		resp, err := getModelWithRetry(t, httpClient, modelID, requestEditor)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		require.Equal(t, "installed", *resp.JSON200.Status, "model should be installed after successful download")
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
		require.Equal(t, "not-installed", *resp.JSON200.Status, "model should not be installed after delete")
	})
}

func newSSEClient(req *http.Request, lastEventID int64) (events chan Event, err error) {

	if lastEventID > 0 {
		req.Header.Set("Last-Event-ID", fmt.Sprintf("%d", lastEventID))
	}
	resp, err := http.DefaultClient.Do(req) //nolint
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("got response status code %d", resp.StatusCode)
	}
	events = make(chan Event)
	go loop(resp.Body, events)
	return events, nil
}

type Event struct {
	ID    string
	Event string
	Data  []byte // json
}

func loop(r io.ReadCloser, events chan Event) {
	defer r.Close()
	reader := bufio.NewReader(r)

	evt := Event{}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			close(events)
			return
		}
		switch {
		case strings.HasPrefix(line, "data:"):
			evt.Data = []byte(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		case strings.HasPrefix(line, "event:"):
			evt.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "id:"):
			evt.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
		case strings.HasPrefix(line, "\n"):
			events <- evt
		default:
			fmt.Fprintf(os.Stderr, "Unknown line: '%s'", line)
			close(events)
		}
	}
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
