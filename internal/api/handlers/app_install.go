// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/arduino/go-paths-helper"
	"github.com/docker/cli/cli/command"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type AppInstallResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Release string `json:"release"`
	Target  string `json:"target"`
}

// HandleAppInstall installs a release archive as a new app. The appID is not an input:
// the release name the archive holds decides it, and it is reported in the "done" event.
func HandleAppInstall(
	dockerClient command.Cli,
	provisioner *orchestrator.Provision,
	idProvider *appid.Provider,
	cfg config.Configuration,
	plat platform.Platform,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, _, err := r.FormFile("file")
		if err != nil {
			slog.Error("missing file parameter", "err", err)
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "missing required file parameter"})
			return
		}
		defer file.Close()

		queryParams := r.URL.Query()
		var prepare bool
		if queryParams.Has("prepare") || queryParams.Get("prepare") == "true" {
			prepare = true
		}

		tempFile, err := paths.MkTempFile(nil, "app-install-*.tar.gz")
		if err != nil {
			slog.Error("unable to create temp file", "err", err)
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "internal server error"})
			return
		}
		tempFilePath := paths.NewFromFile(tempFile)
		defer func() { _ = tempFilePath.Remove() }()

		if _, err := io.Copy(tempFile, file); err != nil {
			tempFile.Close()
			slog.Error("unable to save upload to temp file", "err", err)
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "failed to save uploaded file"})
			return
		}
		tempFile.Close()

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

		result, err := orchestrator.InstallRelease(r.Context(), dockerClient, provisioner, tempFilePath, idProvider, cfg, plat, prepare, func(item orchestrator.StreamMessage) {
			switch item.GetType() {
			case orchestrator.ProgressType:
				sseStream.Send(render.SSEEvent{Type: "progress", Data: progress(*item.GetProgress())})
			case orchestrator.InfoType:
				sseStream.Send(render.SSEEvent{Type: "message", Data: sseLog{Message: item.GetData()}})
			}
		})
		if err != nil {
			code := render.InternalServiceErr
			switch {
			case errors.Is(err, orchestrator.ErrAppAlreadyExists):
				code = "app_already_exists"
			case errors.Is(err, orchestrator.ErrBadRequest):
				code = "bad_request"
			}
			sseStream.SendError(render.SSEErrorData{Code: code, Message: err.Error()})
			return
		}

		sseStream.Send(render.SSEEvent{Type: "done", Data: AppInstallResponse{
			ID:      result.AppID.String(),
			Name:    result.Name,
			Release: result.Release.ID,
			Target:  result.Release.Target,
		}})
	}
}
