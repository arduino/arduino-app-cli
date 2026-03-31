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
	"log/slog"
	"net/http"
	"strconv"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type CreateAppRequest struct {
	Name        string `json:"name" description:"application name" example:"My Awesome App" required:"true"`
	Icon        string `json:"icon" description:"application icon" `
	Description string `json:"description" description:"application description" `
}

func HandleAppCreate(
	idProvider *app.IDProvider,
	cfg config.Configuration,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		defer r.Body.Close()

		queryParams := r.URL.Query()
		skipSketchStr := queryParams.Get("skip-sketch")
		skipSketch := queryParamsValidator(skipSketchStr)

		var req CreateAppRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Error("unable to decode app create request", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "unable to decode app create request"})
			return
		}

		resp, err := orchestrator.CreateApp(
			r.Context(),
			orchestrator.CreateAppRequest{
				Name:        req.Name,
				Icon:        req.Icon,
				Description: req.Description,
				SkipSketch:  skipSketch,
			},
			idProvider,
			cfg,
		)
		if err != nil {
			switch {
			case errors.Is(err, orchestrator.ErrAppAlreadyExists):
				slog.Error("app already exists", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusConflict, models.ErrorResponse{Details: "app already exists"})

			case errors.Is(err, app.ErrInvalidApp):
				slog.Error("invalid app data", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: err.Error()})
			default:
				slog.Error("unable to create app", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "an unexpected error occurred"})
			}
			return
		}
		render.EncodeResponse(w, http.StatusCreated, resp)
	}
}

func queryParamsValidator(param string) bool {
	if param == "" {
		return false
	}
	b, err := strconv.ParseBool(param)
	if err != nil {
		slog.Warn("query value '%q' for AppCreate non valid: %v\n", param, err)
		return false
	}
	return b
}
