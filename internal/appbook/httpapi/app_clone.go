// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/appbook"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type CloneRequest struct {
	Name *string `json:"name" description:"application name" example:"My Awesome App"`
	Icon *string `json:"icon" description:"application icon"`
}

func HandleAppClone(
	idProvider *appid.Provider,
	cfg config.Configuration,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, render.ErrorResponse{Details: "invalid id"})
			return
		}
		defer r.Body.Close()

		var req CloneRequest

		body, err := io.ReadAll(r.Body)
		if err != nil {
			slog.Error("unable to read app clone request", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, render.ErrorResponse{Details: "unable to read app clone request"})
			return
		}
		if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				slog.Error("unable to decode app clone request", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusBadRequest, render.ErrorResponse{Details: "unable to decode app clone request"})
				return
			}
		}

		res, err := appbook.CloneApp(appbook.CloneAppRequest{
			FromID: id,
			Name:   req.Name,
			Icon:   req.Icon,
		}, idProvider, cfg)
		if err != nil {
			if errors.Is(err, appbook.ErrAppAlreadyExists) {
				slog.Error("app already exists", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusConflict, render.ErrorResponse{Details: "app already exists"})
				return
			}
			if errors.Is(err, appbook.ErrAppDoesntExists) {
				slog.Error("app not found", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusNotFound, render.ErrorResponse{Details: "app not found"})
				return
			}
			if errors.Is(err, app.ErrInvalidApp) {
				slog.Error("missing app.yaml", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusBadRequest, render.ErrorResponse{Details: "missing app.yaml"})
				return
			}
			slog.Error("unable to clone app", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, render.ErrorResponse{Details: "unable to clone app"})
			return
		}
		render.EncodeResponse(w, http.StatusCreated, res)
	}
}
