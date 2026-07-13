// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/docker/cli/cli/command"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/render"
)

// HandleAppReleasePrepare pre-pulls the container images an installed app needs so that a
// subsequent start finds them locally. It does not start the app and never downloads models.
func HandleAppReleasePrepare(
	dockerClient command.Cli,
	idProvider *app.IDProvider,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: fmt.Sprintf("invalid id: %s", err.Error())})
			return
		}

		appToPrepare, err := app.Load(id.ToPath())
		if err != nil {
			slog.Error("unable to load the app", "error", err.Error(), "path", id.String())
			if errors.Is(err, os.ErrNotExist) {
				render.EncodeResponse(w, http.StatusNotFound, models.ErrorResponse{Details: err.Error()})
			} else {
				render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: err.Error()})
			}
			return
		}

		if err := orchestrator.PrepareRelease(r.Context(), appToPrepare, dockerClient, func(orchestrator.StreamMessage) {}); err != nil {
			slog.Error("failed to prepare container images", "app_id", id.String(), "error", err)
			if errors.Is(err, orchestrator.ErrBadRequest) {
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: err.Error()})
			} else {
				render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "failed to prepare container images"})
			}
			return
		}

		render.EncodeResponse(w, http.StatusNoContent, nil)
	}
}
