// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package daemon

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/e2e"
	"github.com/arduino/arduino-app-cli/internal/e2e/client"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

// TestAppBuildInvalidTarget asserts the build endpoint rejects an unsupported
// board before any docker work, so it runs on every architecture.
func TestAppReleaseBuildInvalidTarget(t *testing.T) {
	httpClient := GetHttpclient(t)

	createResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{SkipSketch: new(true)},
		client.CreateAppRequest{Icon: new("💻"), Name: "app-to-build"},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	appID := *createResp.JSON201.Id

	resp, err := httpClient.BuildAppWithResponse(t.Context(), appID, client.BuildAppJSONRequestBody{
		Target: new("not-a-board"),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode())

	var body models.ErrorResponse
	require.NoError(t, json.Unmarshal(resp.Body, &body))
	require.Contains(t, body.Details, "the field 'target' must be one of")
}

// TestAppReleaseBuildStream builds a release through the API and asserts the archive is
// streamed back as the response body while the optional build_id tags the build
// events stream. The python environment is built in the runner image, which is
// arm64 only, so a docker daemon is required as well.
func TestAppReleaseBuildStream(t *testing.T) {
	if runtime.GOARCH != ARM64Arch {
		t.Skipf("Skipping test: requires arm64 architecture, currently running on %s", runtime.GOARCH)
	}

	httpClient, daemonAddr := GetHttpclientAndAddr(t, e2e.WithBoardName("unoq"))

	const (
		appName      = "streamed-build-app"
		target       = "unoq"
		releaseLabel = "my-release-label"
		buildID      = "build-42"
	)

	createResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{SkipSketch: new(true)},
		client.CreateAppRequest{Icon: new("💻"), Name: appName},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	appID := *createResp.JSON201.Id

	// The runner image is pulled before the venv is built, so a build is not quick.
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Minute)
	defer cancel()

	// Subscribe to the build events stream before triggering the build: the broker
	// only reaches the subscribers present when an event is published.
	eventsReq, err := http.NewRequestWithContext(ctx, http.MethodGet, daemonAddr+"/v1/apps/build/events", nil)
	require.NoError(t, err)
	events, err := newSSEClient(eventsReq)
	require.NoError(t, err)

	type buildOutcome struct {
		resp *client.BuildAppResp
		err  error
	}
	buildResult := make(chan buildOutcome, 1)
	go func() {
		resp, err := httpClient.BuildAppWithResponse(ctx, appID, client.BuildAppJSONRequestBody{
			Target:       new(target),
			ReleaseLabel: new(releaseLabel),
			BuildId:      new(buildID),
		})
		buildResult <- buildOutcome{resp, err}
	}()

	// The "done" event is published, tagged with the build id, before the archive is
	// streamed, so a subscriber sees the build finish on the stream it filters by id.
	var sawDone bool
	for e := range events {
		t.Log("Received SSE event", "event", e.Event, sseEventData, string(e.Data))
		switch e.Event {
		case sseEventError:
			var payload struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			require.NoError(t, json.Unmarshal(e.Data, &payload))
			// SERVER_CLOSED is emitted by the SSE teardown on every stream; ignore it.
			if payload.Code == sseCodeServerClosed {
				continue
			}
			t.Fatalf("build failed: code=%s message=%s", payload.Code, payload.Message)
		case sseEventDone:
			var payload struct {
				BuildID string `json:"build_id"`
				Name    string `json:"name"`
				Target  string `json:"target"`
			}
			require.NoError(t, json.Unmarshal(e.Data, &payload))
			assert.Equal(t, buildID, payload.BuildID, "the done event must carry the build id")
			assert.Equal(t, target, payload.Target)
			sawDone = true
		}
		if sawDone {
			break
		}
	}
	require.True(t, sawDone, "no done event received on the build events stream")

	outcome := <-buildResult
	require.NoError(t, outcome.err)
	resp := outcome.resp
	require.Equal(t, http.StatusOK, resp.StatusCode())

	// The archive is streamed as a gzip attachment for the client to save.
	require.Contains(t, resp.HTTPResponse.Header.Get("Content-Type"), "gzip")
	disposition := resp.HTTPResponse.Header.Get("Content-Disposition")
	assert.Contains(t, disposition, "attachment")
	assert.Contains(t, disposition, orchestrator.ReleaseArchiveExt)

	// The body is a valid gzip stream: what it ships is asserted by the cli build test.
	gzipReader, err := gzip.NewReader(bytes.NewReader(resp.Body))
	require.NoError(t, err)
	require.NoError(t, gzipReader.Close())
}

func TestAppReleaseInstallFromArchive(t *testing.T) {
	if runtime.GOARCH != "arm64" {
		t.Skipf("Skipping test: requires arm64 architecture, currently running on %s", runtime.GOARCH)
	}

	httpClient, daemonAddr := GetHttpclientAndAddr(t, e2e.WithBoardName("unoq"))

	const (
		appName = "release-install-app"
		target  = "unoq"
	)

	createResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{SkipSketch: new(true)},
		client.CreateAppRequest{Icon: new("💻"), Name: appName},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	appID := *createResp.JSON201.Id

	buildResp, err := httpClient.BuildAppWithResponse(t.Context(), appID, client.BuildAppJSONRequestBody{
		Target: new(target),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, buildResp.StatusCode())

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", appName+".ard")
	require.NoError(t, err)
	_, err = part.Write(buildResp.Body)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	installReq, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPut,
		daemonAddr+"/v1/apps/install?prepare=true",
		body,
	)
	require.NoError(t, err)
	installReq.Header.Set("Content-Type", writer.FormDataContentType())

	events, err := newSSEClient(installReq)
	require.NoError(t, err)

	var sawDone bool
	for e := range events {
		t.Log("Received SSE event", "event", e.Event, sseEventData, string(e.Data))
		switch e.Event {
		case sseEventError:
			var payload struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			require.NoError(t, json.Unmarshal(e.Data, &payload))
			// ignore SERVER_CLOSED
			if payload.Code == sseCodeServerClosed {
				continue
			}
			t.Fatalf("install failed: code=%s message=%s", payload.Code, payload.Message)
		case sseEventDone:
			var payload struct {
				ID      string `json:"id"`
				Name    string `json:"name"`
				Release string `json:"release"`
				Target  string `json:"target"`
			}
			require.NoError(t, json.Unmarshal(e.Data, &payload))
			assert.Equal(t, appName, payload.Name)
			assert.Equal(t, target, payload.Target)
			assert.NotEmpty(t, payload.ID)
			assert.NotEmpty(t, payload.Release)
			sawDone = true
		}
		if sawDone {
			break
		}
	}
	require.True(t, sawDone, "no done event received on the install stream")
}

// TestAppReleasePrepare installs a release without letting install prepare it, then
// asserts the prepare endpoint downloads what it needs on its own.
func TestAppReleasePrepare(t *testing.T) {
	if runtime.GOARCH != ARM64Arch {
		t.Skipf("Skipping test: requires arm64 architecture, currently running on %s", runtime.GOARCH)
	}

	httpClient, daemonAddr := GetHttpclientAndAddr(t, e2e.WithBoardName("unoq"))

	const (
		appName = "release-prepare-app"
		target  = "unoq"
	)

	createResp, err := httpClient.CreateAppWithResponse(
		t.Context(),
		&client.CreateAppParams{SkipSketch: new(true)},
		client.CreateAppRequest{Icon: new("💻"), Name: appName},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	appID := *createResp.JSON201.Id

	buildResp, err := httpClient.BuildAppWithResponse(t.Context(), appID, client.BuildAppJSONRequestBody{
		Target: new(target),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, buildResp.StatusCode())

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", appName+".ard")
	require.NoError(t, err)
	_, err = part.Write(buildResp.Body)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	installReq, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPut,
		daemonAddr+"/v1/apps/install",
		body,
	)
	require.NoError(t, err)
	installReq.Header.Set("Content-Type", writer.FormDataContentType())

	installEvents, err := newSSEClient(installReq)
	require.NoError(t, err)

	var installedID string
	var sawInstallDone bool
	for e := range installEvents {
		t.Log("Received SSE event", "event", e.Event, sseEventData, string(e.Data))
		switch e.Event {
		case sseEventError:
			var payload struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			require.NoError(t, json.Unmarshal(e.Data, &payload))
			if payload.Code == sseCodeServerClosed {
				continue
			}
			t.Fatalf("install failed: code=%s message=%s", payload.Code, payload.Message)
		case sseEventDone:
			var payload struct {
				ID string `json:"id"`
			}
			require.NoError(t, json.Unmarshal(e.Data, &payload))
			installedID = payload.ID
			sawInstallDone = true
		}
		if sawInstallDone {
			break
		}
	}
	require.True(t, sawInstallDone, "no done event received on the install stream")
	require.NotEmpty(t, installedID)

	prepareReq, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPut,
		daemonAddr+"/v1/apps/"+installedID+"/prepare",
		nil,
	)
	require.NoError(t, err)

	prepareEvents, err := newSSEClient(prepareReq)
	require.NoError(t, err)

	var sawPrepareDone bool
	for e := range prepareEvents {
		t.Log("Received SSE event", "event", e.Event, sseEventData, string(e.Data))
		switch e.Event {
		case sseEventError:
			var payload struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			require.NoError(t, json.Unmarshal(e.Data, &payload))
			if payload.Code == sseCodeServerClosed {
				continue
			}
			t.Fatalf("prepare failed: code=%s message=%s", payload.Code, payload.Message)
		case sseEventDone:
			var payload struct {
				ID      string `json:"id"`
				Name    string `json:"name"`
				Release string `json:"release"`
				Target  string `json:"target"`
			}
			require.NoError(t, json.Unmarshal(e.Data, &payload))
			assert.Equal(t, installedID, payload.ID)
			assert.Equal(t, appName, payload.Name)
			assert.Equal(t, target, payload.Target)
			assert.NotEmpty(t, payload.Release)
			sawPrepareDone = true
		}
		if sawPrepareDone {
			break
		}
	}
	require.True(t, sawPrepareDone, "no done event received on the prepare stream")
}
