// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/render"

	"github.com/docker/cli/cli/command"
)

type CloneRequest struct {
	Name *string `json:"name" description:"application name" example:"My Awesome App"`
	Icon *string `json:"icon" description:"application icon"`
}

func HandleAppClone(
	dockerClient command.Cli,
	idProvider *app.IDProvider,
	cfg config.Configuration,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid id"})
			return
		}
		defer r.Body.Close()

		var req CloneRequest

		body, err := io.ReadAll(r.Body)
		if err != nil {
			slog.Error("unable to read app clone request", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "unable to read app clone request"})
			return
		}
		if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				slog.Error("unable to decode app clone request", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "unable to decode app clone request"})
				return
			}
		}

		res, err := orchestrator.CloneApp(r.Context(), orchestrator.CloneAppRequest{
			FromID: id,
			Name:   req.Name,
			Icon:   req.Icon,
		}, idProvider, cfg)
		if err != nil {
			if errors.Is(err, orchestrator.ErrAppAlreadyExists) {
				slog.Error("app already exists", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusConflict, models.ErrorResponse{Details: "app already exists"})
				return
			}
			if errors.Is(err, orchestrator.ErrAppDoesntExists) {
				slog.Error("app not found", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusNotFound, models.ErrorResponse{Details: "app not found"})
				return
			}
			if errors.Is(err, app.ErrInvalidApp) {
				slog.Error("missing app.yaml", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "missing app.yaml"})
				return
			}
			slog.Error("unable to clone app", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to clone app"})
			return
		}
		render.EncodeResponse(w, http.StatusCreated, res)
	}
}
