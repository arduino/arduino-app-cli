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
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/docker/cli/cli/command"

	"github.com/arduino/arduino-app-cli/internal/api/edgeimpulse"
	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type InstallEIModelRequest struct {
	ImpulseID *int `json:"impulse_id" description:"Edge Impulse impulse ID" example:"1" required:"true"`
}

func HandleModelsList(modelsIndex *modelsindex.ModelsIndex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := r.URL.Query()

		var brickFilter []string
		if brick := params.Get("bricks"); brick != "" {
			brickFilter = strings.Split(strings.TrimSpace(brick), ",")
		}
		res := orchestrator.AIModelsList(orchestrator.AIModelsListRequest{
			FilterByBrickID: brickFilter,
		}, modelsIndex)
		render.EncodeResponse(w, http.StatusOK, res)
	}
}

func HandlerModelByID(modelsIndex *modelsindex.ModelsIndex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("modelID")
		if id == "" {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "id must be set"})
			return
		}
		res, found := orchestrator.AIModelDetails(modelsIndex, id)
		if !found {
			details := fmt.Sprintf("models with id %q not found", id)
			render.EncodeResponse(w, http.StatusNotFound, models.ErrorResponse{Details: details})
			return
		}
		render.EncodeResponse(w, http.StatusOK, res)
	}
}

func HandlerDeleteModelByID(dockerClient command.Cli, cfg config.Configuration, modelsIndex *modelsindex.ModelsIndex, idProvider *app.IDProvider, platform platform.Platform) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.PathValue("modelID"))
		if id == "" {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "id must be set"})
			return
		}
		forceRaw := r.URL.Query().Get("force")
		force, err := strconv.ParseBool(forceRaw)
		if err != nil {
			force = false
		}

		err = orchestrator.AIModelDelete(r.Context(), dockerClient, cfg, modelsIndex, platform, id, idProvider, force)
		if err != nil {
			switch {
			case errors.Is(err, orchestrator.ErrNotFound):
				render.EncodeResponse(w, http.StatusNotFound, models.ErrorResponse{Details: err.Error()})
			case errors.Is(err, orchestrator.ErrConflict):
				render.EncodeResponse(w, http.StatusConflict, models.ErrorResponse{Details: err.Error()})
			case errors.Is(err, orchestrator.ErrCannotRemoveModel):
				render.EncodeResponse(w, http.StatusConflict, models.ErrorResponse{Details: err.Error()})
			default:
				render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: err.Error()})
			}
			return
		}

		render.EncodeResponse(w, http.StatusNoContent, nil)
	}
}

func HandleInstallEIModel(cfg config.Configuration, bricksIndex *bricksindex.BricksIndex, modelsIndex *modelsindex.ModelsIndex, dockerClient command.Cli) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID, err := strconv.Atoi(r.PathValue("projectID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "invalid projectID"})
			return
		}
		prjApiKey := r.Header.Get("x-api-key")
		if prjApiKey == "" {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "x-api-key header must be set"})
			return
		}

		var req InstallEIModelRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Error("unable to decode download EI model request", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "unable to decode download EI model request"})
			return
		}

		if err := req.Validate(); err != nil {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: err.Error()})
			return
		}

		eiClient, err := edgeimpulse.NewEIClient(prjApiKey, *cfg.EdgeImpulseAPIURL)
		if err != nil {
			slog.Error("unable to create Edge Impulse client", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to create Edge Impulse client"})
			return
		}

		eiModel, err := orchestrator.InstallEIModel(r.Context(), bricksIndex, modelsIndex, dockerClient, eiClient, cfg.CustomModelsDir(), projectID, *req.ImpulseID)
		if err != nil {
			switch {
			case errors.Is(err, edgeimpulse.ErrUnauthorized):
				slog.Error("unauthorized access to Edge Impulse model", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusUnauthorized, models.ErrorResponse{Details: "unauthorized access to Edge Impulse model"})
				return
			case errors.Is(err, orchestrator.ErrIncompleteImpulse):
				slog.Error("incomplete impulse for Edge Impulse model", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "incomplete impulse for Edge Impulse model"})
				return
			case errors.Is(err, edgeimpulse.ErrForbidden):
				slog.Error("forbidden access to Edge Impulse model", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusForbidden, models.ErrorResponse{Details: "forbidden access to Edge Impulse model"})
				return
			case errors.Is(err, orchestrator.ErrInsufficientStorage):
				slog.Error("insufficient storage to install Edge Impulse model", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusInsufficientStorage, models.ErrorResponse{Details: "insufficient storage to install Edge Impulse model"})
				return
			default:
				slog.Error("unable to install Edge Impulse model", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to install Edge Impulse model: " + err.Error()})
				return
			}
		}

		// FIXME: read the installed model using the modelindex.getModelByID
		render.EncodeResponse(w, http.StatusOK, eiModel)
	}
}

func (r InstallEIModelRequest) Validate() error {
	if r.ImpulseID == nil || *r.ImpulseID <= 0 {
		return fmt.Errorf("impulse_id must be an integer greater than 0")
	}
	return nil
}
