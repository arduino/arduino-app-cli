// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
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
	BuildID      string `json:"build_id" description:"optional build identifier used to tag the build events stream"`
	Target       string `json:"target" description:"board target to build for"`
	ReleaseLabel string `json:"release_label" description:"optional label to attach to the release"`
	IncludeData  bool   `json:"include_data" description:"include the app data in the release"`
	ReleaseNotes string `json:"notes" description:"notes to attach to the release"`
}

// HandleAppBuild builds an app into a release archive and streams the archive
// back as the response body. Build progress is published to the global build events stream.
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

		appToBuild, err := app.Load(id.ToPath())
		if err != nil {
			slog.Error("unable to load the app", slog.String("error", err.Error()), slog.String("path", id.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to load the app"})
			return
		}

		var buildReq buildRequest
		if err := json.NewDecoder(r.Body).Decode(&buildReq); err != nil {
			slog.Error("unable to decode app build request", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "unable to decode app build request"})
			return
		}

		buildID := buildReq.BuildID

		if buildReq.Target != "" {
			if _, ok := platform.ForBoard(buildReq.Target); !ok {
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: fmt.Sprintf("the field 'target' must be one of %s", strings.Join(platform.SupportedBoards(), ", "))})
				return
			}
		}

		req := orchestrator.BuildReleaseRequest{
			Target:       buildReq.Target,
			ReleaseLabel: buildReq.ReleaseLabel,
			Notes:        buildReq.ReleaseNotes,
			IncludeData:  buildReq.IncludeData,
		}
		result, reader, err := orchestrator.BuildRelease(r.Context(), dockerClient, provisioner, appToBuild, req, cfg, func(item orchestrator.StreamMessage) {
			broker.PublishStreamMessage(buildID, item)
		})
		if err != nil {
			slog.Error("Unable to build the app", slog.String("error", err.Error()))
			code := render.InternalServiceErr
			status := http.StatusInternalServerError
			if errors.Is(err, orchestrator.ErrBadRequest) {
				code = "BAD_REQUEST"
				status = http.StatusBadRequest
			}
			broker.PublishError(buildID, code, err.Error())
			render.EncodeResponse(w, status, models.ErrorResponse{Details: err.Error()})
			return
		}
		// Closing the stream removes the build staging dir, whatever happens to the archive.
		defer func() { _ = reader.Close() }()

		// The archive is drained to a file before it is served: a late write error is still
		// answered with a status and a body, not a truncated download on a 200 already sent.
		artifactsDir := paths.New(os.TempDir(), "build-artifacts")
		if err := artifactsDir.MkdirAll(); err != nil {
			slog.Error("unable to create the build artifacts dir", slog.String("error", err.Error()))
			broker.PublishError(buildID, render.InternalServiceErr, "unable to prepare the build output directory")
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to prepare the build output directory"})
			return
		}
		archivePath := artifactsDir.Join(result.FileName)
		defer func() { _ = archivePath.Remove() }()

		if err := releasebuild.WriteArchive(reader, archivePath); err != nil {
			slog.Error("unable to write the release archive", slog.String("error", err.Error()))
			broker.PublishError(buildID, render.InternalServiceErr, "unable to write the release archive")
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to write the release archive"})
			return
		}

		broker.PublishDone(buildID, result.Name, result.Target)

		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, result.FileName))
		http.ServeFile(w, r, archivePath.String())
	}
}

// HandleAppBuildEvents streams, as Server-Sent Events, the progress of every
// build of every app. Each event carries the "build_id" it belongs to, so a
// client can filter the stream down to a single build it triggered.
func HandleAppBuildEvents(broker *releasebuild.EventBroker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sseStream, err := render.NewSSEStream(r.Context(), w)
		if err != nil {
			slog.Error("unable to create SSE stream", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to create SSE stream"})
			return
		}
		defer sseStream.Close()

		events, unsubscribe := broker.Subscribe()
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
