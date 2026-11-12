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
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app/generator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type AppLocalBrickCreateRequest struct {
	Name string `json:"name"`
}

type AppLocalBrickCreateResponse struct {
	ID string `json:"id"`
}

func HandleAppLocalBrickCreate(idProvider *appid.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appId, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid app id"})
			return
		}
		a, ok := loadEditableApp(w, appId)
		if !ok {
			return
		}

		var req AppLocalBrickCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Error("Failed to decode request body", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "invalid request body"})
			return
		}
		if req.Name == "" {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "name is required"})
			return
		}

		id, err := generator.BrickIDFromName(req.Name)
		if err != nil {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: err.Error()})
			return
		}

		err = generator.GenerateLocalBrick(a.GetBricksPath(), id, req.Name)
		if err != nil {
			if errors.Is(err, generator.ErrBrickAlreadyExists) {
				render.EncodeResponse(w, http.StatusConflict, models.ErrorResponse{Details: fmt.Sprintf("a brick with the same id '%s' already exists", id)})
				return
			}
			slog.Error("Failed to generate local brick", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "failed to generate local brick"})
			return
		}

		render.EncodeResponse(w, http.StatusCreated, AppLocalBrickCreateResponse{ID: id})
	}
}
