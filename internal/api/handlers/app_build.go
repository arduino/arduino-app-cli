// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/arduino/go-paths-helper"
	"github.com/docker/cli/cli/command"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-app-cli/internal/releasebuild"
	"github.com/arduino/arduino-app-cli/internal/render"
)

// buildRequest is the JSON body of a build request.
type buildRequest struct {
	Target       string `json:"target" description:"board target to build for"`
	IncludeData  bool   `json:"include_data" description:"include the app data in the release"`
	ReleaseNotes string `json:"notes" description:"notes to attach to the release"`
}

// The build progress events published to the app-wide build events stream. Each
// carries the build id it belongs to, so a subscriber can filter by build. The
// id is empty when the caller did not provide the "buildid" query parameter.
type (
	buildProgressEvent struct {
		BuildID  string  `json:"build_id,omitempty"`
		Name     string  `json:"name"`
		Progress float32 `json:"progress"`
	}
	buildMessageEvent struct {
		BuildID string `json:"build_id,omitempty"`
		Message string `json:"message"`
	}
	buildDoneEvent struct {
		BuildID string `json:"build_id,omitempty"`
		Name    string `json:"name"`
		Target  string `json:"target"`
	}
	buildErrorEvent struct {
		BuildID string            `json:"build_id,omitempty"`
		Code    render.SSEErrCode `json:"code"`
		Message string            `json:"message,omitempty"`
	}
)

// HandleAppBuild builds an app into a release archive and streams the archive
// back as the response body. Build progress is published to the app-wide build events stream.
func HandleAppBuild(
	dockerClient command.Cli,
	provisioner *orchestrator.Provision,
	idProvider *appid.Provider,
	cfg config.Configuration,
	broker *releasebuild.EventBroker,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid id"})
			return
		}

		appKey := id.String()
		buildID := r.URL.Query().Get("buildid")

		appToBuild, err := app.Load(id.ToPath())
		if err != nil {
			slog.Error("unable to load the app", slog.String("error", err.Error()), slog.String("path", id.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to load the app"})
			return
		}

		defer r.Body.Close()

		var buildReq buildRequest
		body, err := io.ReadAll(r.Body)
		if err != nil {
			slog.Error("unable to read app build request", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "unable to read app build request"})
			return
		}
		if len(body) > 0 {
			if err := json.Unmarshal(body, &buildReq); err != nil {
				slog.Error("unable to decode app build request", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "unable to decode app build request"})
				return
			}
		}

		if buildReq.Target != "" {
			if _, ok := platform.ForBoard(buildReq.Target); !ok {
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: fmt.Sprintf("the field 'target' must be one of %s", strings.Join(platform.SupportedBoards(), ", "))})
				return
			}
		}

		// Finished archives live here
		// TODO 2 to be defined how it should work. handle deletion/TTL and so on...
		artifactsDir := buildArtifactsDir()
		if err := artifactsDir.MkdirAll(); err != nil {
			slog.Error("unable to create the build artifacts dir", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to prepare the build output directory"})
			return
		}

		req := orchestrator.BuildReleaseRequest{
			Target:      buildReq.Target,
			Notes:       buildReq.ReleaseNotes,
			IncludeData: buildReq.IncludeData,
			Output:      artifactsDir,
			Overwrite:   true,
		}

		result, err := orchestrator.BuildRelease(r.Context(), dockerClient, provisioner, appToBuild, req, cfg, func(item orchestrator.StreamMessage) {
			// A StreamMessage may carry a progress value, an info message, or both,
			// so publish each independently to avoid dropping either one.
			if p := item.GetProgress(); p != nil {
				broker.Publish(appKey, render.SSEEvent{Type: "progress", Data: buildProgressEvent{BuildID: buildID, Name: p.Name, Progress: p.Progress}})
			}
			if item.GetData() != "" {
				broker.Publish(appKey, render.SSEEvent{Type: "message", Data: buildMessageEvent{BuildID: buildID, Message: item.GetData()}})
			}
		})
		if err != nil {
			slog.Error("Unable to build the app", slog.String("error", err.Error()))
			code := render.InternalServiceErr
			status := http.StatusInternalServerError
			if errors.Is(err, orchestrator.ErrBadRequest) {
				code = "BAD_REQUEST"
				status = http.StatusBadRequest
			}
			broker.Publish(appKey, render.SSEEvent{Type: "error", Data: buildErrorEvent{BuildID: buildID, Code: code, Message: err.Error()}})
			render.EncodeResponse(w, status, models.ErrorResponse{Details: err.Error()})
			return
		}

		archivePath := paths.New(result.Archive)
		defer func() { _ = archivePath.Remove() }()

		broker.Publish(appKey, render.SSEEvent{Type: "done", Data: buildDoneEvent{BuildID: buildID, Name: result.Name, Target: result.Target}})

		if !archivePath.Exist() {
			slog.Error("the build archive is missing", slog.String("path", archivePath.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "the build archive is missing"})
			return
		}

		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, archivePath.Base()))
		http.ServeFile(w, r, archivePath.String())
	}
}

// HandleAppBuildEvents streams, as Server-Sent Events, the progress of every
// build of the given app. Each event carries the "build_id" it belongs to, so a
// client can filter the stream down to a single build it triggered.
func HandleAppBuildEvents(idProvider *appid.Provider, broker *releasebuild.EventBroker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid id"})
			return
		}

		sseStream, err := render.NewSSEStream(r.Context(), w)
		if err != nil {
			slog.Error("unable to create SSE stream", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to create SSE stream"})
			return
		}
		defer sseStream.Close()

		events, unsubscribe := broker.Subscribe(id.String())
		defer unsubscribe()

		for {
			select {
			case <-r.Context().Done():
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				sseStream.Send(event)
			}
		}
	}
}

func buildArtifactsDir() *paths.Path {
	return paths.New(os.TempDir(), "build-artifacts")
}
