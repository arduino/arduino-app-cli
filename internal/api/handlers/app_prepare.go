// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/docker/cli/cli/command"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type AppPrepareResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Release string `json:"release"`
	Target  string `json:"target"`
}

// HandleAppPrepare downloads what an already installed release needs to run,
// without starting it: the containers its compose file names and the models it ships.
func HandleAppPrepare(
	dockerClient command.Cli,
	provisioner *orchestrator.Provision,
	idProvider *appid.Provider,
	cfg config.Configuration,
	plat platform.Platform,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid id"})
			return
		}

		arduinoApp, err := app.Load(id.ToPath())
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", id.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
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

		if err := orchestrator.PrepareInstalledRelease(r.Context(), dockerClient, provisioner, arduinoApp, cfg, plat, func(item orchestrator.StreamMessage) {
			switch item.GetType() {
			case orchestrator.ProgressType:
				sseStream.Send(render.SSEEvent{Type: "progress", Data: progress(*item.GetProgress())})
			case orchestrator.InfoType:
				sseStream.Send(render.SSEEvent{Type: "message", Data: sseLog{Message: item.GetData()}})
			}
		}); err != nil {
			code := render.InternalServiceErr
			if errors.Is(err, orchestrator.ErrBadRequest) {
				code = "bad_request"
			}
			sseStream.SendError(render.SSEErrorData{Code: code, Message: err.Error()})
			return
		}

		release, _ := arduinoApp.GetRelease()
		sseStream.Send(render.SSEEvent{Type: "done", Data: AppPrepareResponse{
			ID:      id.String(),
			Name:    arduinoApp.Name,
			Release: release.ID,
			Target:  release.Target,
		}})
	}
}
