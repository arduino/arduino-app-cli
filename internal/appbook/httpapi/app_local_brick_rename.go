// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricks"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type AppLocalBrickRenameRequest struct {
	Name string `json:"name"`
}

func HandleAppLocalBrickRename(brickService *bricks.Service, idProvider *appid.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appId, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, render.ErrorResponse{Details: "invalid app id"})
			return
		}
		a, ok := LoadEditableApp(w, appId)
		if !ok {
			return
		}

		oldID := r.PathValue("brickID")
		if oldID == "" {
			render.EncodeResponse(w, http.StatusBadRequest, render.ErrorResponse{Details: "brickID must be set"})
			return
		}

		var req AppLocalBrickRenameRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Error("Failed to decode request body", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, render.ErrorResponse{Details: "invalid request body"})
			return
		}
		if req.Name == "" {
			render.EncodeResponse(w, http.StatusBadRequest, render.ErrorResponse{Details: "name is required"})
			return
		}

		newID, err := generateBrickID(req.Name)
		if err != nil {
			render.EncodeResponse(w, http.StatusBadRequest, render.ErrorResponse{Details: err.Error()})
			return
		}

		res, err := brickService.LocalBrickRename(a, oldID, newID, req.Name)
		if err != nil {
			switch {
			case errors.Is(err, bricks.ErrBrickNotFound):
				render.EncodeResponse(w, http.StatusNotFound, render.ErrorResponse{Details: fmt.Sprintf("brick %q not found", oldID)})
			case errors.Is(err, bricks.ErrBrickNotLocal):
				render.EncodeResponse(w, http.StatusBadRequest, render.ErrorResponse{Details: "only local bricks can be renamed"})
			case errors.Is(err, bricks.ErrBrickIDConflict):
				render.EncodeResponse(w, http.StatusConflict, render.ErrorResponse{Details: fmt.Sprintf("a brick with id %q already exists", newID)})
			default:
				slog.Error("Failed to rename local brick", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusInternalServerError, render.ErrorResponse{Details: "failed to rename local brick"})
			}
			return
		}

		render.EncodeResponse(w, http.StatusOK, res)
	}
}
