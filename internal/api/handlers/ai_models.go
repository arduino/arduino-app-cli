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
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/arduino/arduino-app-cli/internal/api/edgeimpulse"
	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type InstallEIModelRequest struct {
	ProjectID int    `json:"project_id" description:"Edge Impulse project ID" example:"123456" required:"true"`
	ImpulseID int    `json:"impulse_id" description:"Edge Impulse impulse ID" example:"1" required:"true"`
	PrjApiKey string `json:"prj_api_key" description:"Edge Impulse API token" example:"your_edge_impulse_api_token" required:"true"`
	ModelType string `json:"model_type" description:"Type of model to build" example:"int8" required:"true"`
	// Engine     string `json:"engine" description:"Model engine (e.g., 'tensorflow-lite')" example:"tensorflow-lite" required:"true"`
	// DeviceType string `json:"device_type" description:"Device type for deployment (e.g., 'arduino-uno-q')" example:"arduino-uno-q" required:"true"`
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

func HandleInstallEIModel(cfg config.Configuration, bricksIndex *bricksindex.BricksIndex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req InstallEIModelRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Error("unable to decode app create request", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "unable to decode app create request"})
			return
		}
		defer r.Body.Close()

		if cfg.EdgeImpulseAPIURL == nil {
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "Edge Impulse API URL is not configured"})
			return
		}

		httpClient, err := edgeimpulse.NewClientWithResponses((*cfg.EdgeImpulseAPIURL).String(), edgeimpulse.WithRequestEditorFn(func(ctx context.Context, ri *http.Request) error {
			ri.Header.Add("x-api-key", req.PrjApiKey)
			ri.Header.Set("Content-Type", "application/json")
			return nil
		}))
		if err != nil {
			slog.Error("unable to create Edge Impulse client", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to create Edge Impulse client"})
			return
		}

		eiModel, err := orchestrator.InstallEIModel(r.Context(),
			bricksIndex,
			edgeimpulse.NewProjectDownloader(httpClient, req.ProjectID),
			cfg.CustomModelsDir(),
			edgeimpulse.ModelTypeParameter(req.ModelType),
			&req.ImpulseID,
		)
		if err != nil {
			slog.Error("unable to  download model", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to create Edge Impulse client"})
			return
			// switch {
			// case errors.Is(err, edgeimpulse.UnauthorizedErr):
			// 	slog.Error("unauthorized access to Edge Impulse API", slog.String("error", err.Error()))
			// 	render.EncodeResponse(w, http.StatusUnauthorized, models.ErrorResponse{Details: "unauthorized access to Edge Impulse API"})
			// 	return
			// default:
			// 	slog.Error("unable to install Edge Impulse model", slog.String("error", err.Error()))
			// 	render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to install Edge Impulse model"})
			// 	return
			// }
		}

		// FIXME: read the installed model using the modelindex.getModelByID
		render.EncodeResponse(w, http.StatusOK, eiModel)
	}
}
