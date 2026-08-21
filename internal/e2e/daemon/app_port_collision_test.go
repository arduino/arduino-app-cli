// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/e2e/client"
)

// TestAppStartPortCollision checks that an app declaring the same port from more than one
// source is refused, and that the error names every source involved.

func TestAppStartPortCollision(t *testing.T) {
	httpClient, daemonAddr := GetHttpclientAndAddr(t)

	// arduino:web_ui and arduino:streamlit_ui both declare port 7000 in the brick index.
	t.Run("bricks running inside python runner", func(t *testing.T) {
		message := startAppAndExpectError(t, httpClient, daemonAddr, "python-runner-collision",
			"arduino:web_ui", "arduino:streamlit_ui")

		require.Contains(t, message, "port 7000 is declared by more than one source")
		require.Contains(t, message, "arduino:web_ui")
		require.Contains(t, message, "arduino:streamlit_ui")
	})

	// arduino:video_image_classification and arduino:video_object_detection publish host port
	// 4912 from their own brick_compose.yaml.
	t.Run("bricks running in their own containers", func(t *testing.T) {
		message := startAppAndExpectError(t, httpClient, daemonAddr, "collision-between-containers",
			"arduino:video_image_classification", "arduino:video_object_detection")

		require.Contains(t, message, "port 4912 is declared by more than one source")
		require.Contains(t, message, "arduino:video_image_classification")
		require.Contains(t, message, "arduino:video_object_detection")
	})
}

func startAppAndExpectError(
	t *testing.T,
	httpClient *client.ClientWithResponses,
	daemonAddr string,
	appName string,
	brickIDs ...string,
) string {
	t.Helper()
	noEditor := func(ctx context.Context, req *http.Request) error { return nil }

	createResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{SkipSketch: new(true)},
		client.CreateAppRequest{Icon: new("💻"), Name: appName},
		noEditor,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	require.NotNil(t, createResp.JSON201)
	appID := *createResp.JSON201.Id

	for _, brickID := range brickIDs {
		respBrick, err := httpClient.UpsertAppBrickInstanceWithResponse(
			t.Context(), appID, brickID, client.BrickCreateUpdateRequest{}, noEditor,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, respBrick.StatusCode(), "failed to add brick %s", brickID)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, daemonAddr+"/v1/apps/"+appID+"/start", nil)
	require.NoError(t, err)
	events, err := newSSEClient(req, 0)
	require.NoError(t, err)

	var errorMessages []string
	for e := range events {
		t.Log("Received SSE event", "event", e.Event, "data", string(e.Data))
		if e.Event != "error" {
			continue
		}
		var payload struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		require.NoError(t, json.Unmarshal(e.Data, &payload))
		if payload.Code == "SERVER_CLOSED" {
			// Emitted when the stream is torn down, not a failure of the start itself.
			continue
		}
		errorMessages = append(errorMessages, payload.Message)
	}

	require.Len(t, errorMessages, 1, "expected the start to fail with a single error")
	return errorMessages[0]
}
