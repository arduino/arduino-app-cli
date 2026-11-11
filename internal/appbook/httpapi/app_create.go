// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/arduino/arduino-app-cli/internal/appbook"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type CreateAppRequest struct {
	Name        string `json:"name" description:"application name" example:"My Awesome App" required:"true"`
	Icon        string `json:"icon" description:"application icon" `
	Description string `json:"description" description:"application description" `
}

func HandleAppCreate(
	bricksIndex *bricksindex.BricksIndex,
	idProvider *appid.Provider,
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
			render.EncodeResponse(w, http.StatusBadRequest, render.ErrorResponse{Details: "unable to decode app create request"})
			return
		}

		resp, err := appbook.CreateApp(
			appbook.CreateAppRequest{
				Name:        req.Name,
				Icon:        req.Icon,
				Description: req.Description,
				SkipSketch:  skipSketch,
			},
			bricksIndex,
			idProvider,
			cfg,
		)
		if err != nil {
			switch {
			case errors.Is(err, appbook.ErrAppAlreadyExists):
				slog.Error("app already exists", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusConflict, render.ErrorResponse{Details: err.Error()})

			case errors.Is(err, app.ErrInvalidApp):
				slog.Error("invalid app data", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusBadRequest, render.ErrorResponse{Details: err.Error()})
			default:
				slog.Error("unable to create app", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusInternalServerError, render.ErrorResponse{Details: "an unexpected error occurred"})
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
