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

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricks"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type AppLocalBrickRenameRequest struct {
	Name string `json:"name"`
}

func HandleAppLocalBrickRename(brickService *bricks.Service, idProvider *app.IDProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appId, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid app id"})
			return
		}

		a, err := app.Load(appId.ToPath())
		if err != nil {
			slog.Error("Unable to load the app", slog.String("error", err.Error()), slog.String("path", appId.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}

		oldID := r.PathValue("brickID")
		if oldID == "" {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "brickID must be set"})
			return
		}

		var req AppLocalBrickRenameRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Error("Failed to decode request body", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "invalid request body"})
			return
		}
		if req.Name == "" {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "name is required"})
			return
		}

		newID, err := generateBrickID(req.Name)
		if err != nil {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: err.Error()})
			return
		}

		res, err := brickService.LocalBrickRename(&a, oldID, newID, req.Name)
		if err != nil {
			switch {
			case errors.Is(err, bricks.ErrBrickNotFound):
				render.EncodeResponse(w, http.StatusNotFound, models.ErrorResponse{Details: fmt.Sprintf("brick %q not found", oldID)})
			case errors.Is(err, bricks.ErrBrickNotLocal):
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "only local bricks can be renamed"})
			case errors.Is(err, bricks.ErrBrickIDConflict):
				render.EncodeResponse(w, http.StatusConflict, models.ErrorResponse{Details: fmt.Sprintf("a brick with id %q already exists", newID)})
			default:
				slog.Error("Failed to rename local brick", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "failed to rename local brick"})
			}
			return
		}

		render.EncodeResponse(w, http.StatusOK, res)
	}
}
