// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app/generator"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type AppLocalBrickCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type AppLocalBrickCreateResponse struct {
	ID string `json:"id"`
}

func HandleAppLocalBrickCreate(idProvider *app.IDProvider) http.HandlerFunc {
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

		id, err := generateBrickID(req.Name)
		if err != nil {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: err.Error()})
			return
		}

		err = generator.GenerateLocalBrick(a.GetBricksPath(), id, req.Name, req.Description)
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

var brickIDRegexp = regexp.MustCompile(`[^a-z0-9]+`)

func generateBrickID(name string) (string, error) {
	if strings.Contains(name, ".") {
		return "", errors.New("brick name cannot contain '.' character")
	}
	if strings.Contains(name, ":") {
		return "", errors.New("brick name cannot contain ':' character")
	}

	id := strings.ToLower(name)
	id = brickIDRegexp.ReplaceAllString(id, "_")
	id = strings.Trim(id, "_")
	if id == "" {
		return "", errors.New("brick name must contain at least one alphanumeric character")
	}
	return id, nil
}
