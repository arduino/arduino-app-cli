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
	"github.com/arduino/arduino-app-cli/internal/render"
)

// buildRequest is the JSON body of a build request.
type buildRequest struct {
	Target       string `json:"target" description:"board target to build for"`
	IncludeData  bool   `json:"include_data" description:"include the app data in the release"`
	ReleaseNotes string `json:"notes" description:"notes to attach to the release"`
}

// buildArtifact is the "done" event of a build: the release facts plus where the
// archive can be downloaded. artifact_id is the archive file name, and download_path
// is the endpoint that serves it.
type buildArtifact struct {
	Name         string `json:"name"`
	Target       string `json:"target"`
	ArtifactID   string `json:"artifact_id"`
	DownloadPath string `json:"download_path"`
}

// HandleAppBuild builds an app into a release archive and
// reports it as a stream of events.
// The final "done" event streams the artifact location.
func HandleAppBuild(
	dockerClient command.Cli,
	provisioner *orchestrator.Provision,
	idProvider *appid.Provider,
	cfg config.Configuration,
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
			Output:      buildArtifactsDir(),
			Overwrite:   true,
		}

		sseStream, err := render.NewSSEStream(r.Context(), w)
		if err != nil {
			slog.Error("unable to create SSE stream", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to create SSE stream"})
			return
		}
		defer sseStream.Close()

		type progress struct {
			Name     string  `json:"name"`
			Progress float32 `json:"progress"`
		}
		type message struct {
			Message string `json:"message"`
		}

		result, err := orchestrator.BuildRelease(r.Context(), dockerClient, provisioner, appToBuild, req, cfg, func(item orchestrator.StreamMessage) {
			// A StreamMessage may carry a progress value, an info message, or both,
			// so emit each independently to avoid dropping either one.
			if p := item.GetProgress(); p != nil {
				sseStream.Send(render.SSEEvent{Type: "progress", Data: progress(*p)})
			}
			if item.GetData() != "" {
				sseStream.Send(render.SSEEvent{Type: "message", Data: message{Message: item.GetData()}})
			}
		})
		if err != nil {
			slog.Error("Unable to build the app", slog.String("error", err.Error()))
			code := render.InternalServiceErr
			if errors.Is(err, orchestrator.ErrBadRequest) {
				code = "BAD_REQUEST"
			}
			sseStream.SendError(render.SSEErrorData{Code: code, Message: err.Error()})
			return
		}

		// The id is the archive file name; the download path is the endpoint that
		// serves it back. Anyone with the appID can retrieve it afterwards.
		artifactID := paths.New(result.Archive).Base()
		sseStream.Send(render.SSEEvent{Type: "done", Data: buildArtifact{
			Name:         result.Name,
			Target:       result.Target,
			ArtifactID:   artifactID,
			DownloadPath: fmt.Sprintf("/v1/apps/%s/build/%s", r.PathValue("appID"), artifactID),
		}})
	}
}

// HandleAppBuildArtifact serves a release archive a previous build produced. The
// artifactID is the archive file name reported in the build's "done" event.
func HandleAppBuildArtifact(idProvider *appid.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := idProvider.IDFromBase64(r.PathValue("appID")); err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid id"})
			return
		}

		artifactID := r.PathValue("artifactID")
		// The id names a file in the artifacts dir and nothing else: no separators, no
		// traversal, and the release extension, so it can only resolve inside the dir.
		if artifactID == "" ||
			strings.ContainsAny(artifactID, `/\`) ||
			strings.Contains(artifactID, "..") ||
			!strings.HasSuffix(artifactID, orchestrator.ReleaseArchiveExt) {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "invalid artifact id"})
			return
		}

		artifactPath := buildArtifactPath(artifactID)
		if !artifactPath.Exist() {
			render.EncodeResponse(w, http.StatusNotFound, models.ErrorResponse{Details: "artifact not found"})
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, artifactID))
		http.ServeFile(w, r, artifactPath.String())
	}
}

func buildArtifactsDir() *paths.Path {
	return paths.New(os.TempDir(), "build-artifacts")
}

func buildArtifactPath(artifactID string) *paths.Path {
	return paths.New(buildArtifactsDir().String(), artifactID)
}
