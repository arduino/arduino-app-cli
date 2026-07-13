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
		res := orchestrator.AIModelsList(r.Context(), orchestrator.AIModelsListRequest{
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
		res, found, err := orchestrator.AIModelDetails(r.Context(), modelsIndex, id)
		if err != nil {
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: err.Error()})
			return
		}
		if !found {
			details := fmt.Sprintf("models with id %q not found", id)
			render.EncodeResponse(w, http.StatusNotFound, models.ErrorResponse{Details: details})
			return
		}
		render.EncodeResponse(w, http.StatusOK, res)
	}
}

func HandlerDeleteModelByID(dockerClient command.Cli, cfg config.Configuration, modelsIndex *modelsindex.ModelsIndex, bricksIndex *bricksindex.BricksIndex, idProvider *app.IDProvider, platform platform.Platform) http.HandlerFunc {
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

		err = orchestrator.AIModelDelete(r.Context(), dockerClient, cfg, modelsIndex, bricksIndex, platform, id, idProvider, force)
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

func HandleInstallEIModel(cfg config.Configuration, bricksIndex *bricksindex.BricksIndex, modelsIndex *modelsindex.ModelsIndex, dockerClient command.Cli, platform platform.Platform) http.HandlerFunc {
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

		eiModel, err := orchestrator.InstallEIModel(r.Context(), bricksIndex, modelsIndex, dockerClient, eiClient, cfg.CustomModelsDir(), platform, projectID, *req.ImpulseID)
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

func HandleInstallModel(dockerClient command.Cli, modelsIndex *modelsindex.ModelsIndex, plat platform.Platform) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.PathValue("modelID"))
		if id == "" {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "model ID must be set"})
			return
		}

		model, err := modelsIndex.GetModelByID(r.Context(), id)
		if err != nil {
			slog.Error("unable to get model by ID", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to get model by ID"})
			return
		}
		if model == nil {
			render.EncodeResponse(w, http.StatusNotFound, models.ErrorResponse{Details: fmt.Sprintf("model %q not found", id)})
			return
		}
		if model.Status == modelsindex.InstalledStatus {
			render.EncodeResponse(w, http.StatusConflict, models.ErrorResponse{Details: fmt.Sprintf("model %q already installed", id)})
			return
		}

		sseStream, err := render.NewSSEStream(r.Context(), w)
		if err != nil {
			slog.Error("unable to create SSE stream", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to create SSE stream"})
			return
		}
		defer sseStream.Close()

		type progress struct {
			Name     string  `json:"name"`
			Total    int64   `json:"total"`
			Current  int64   `json:"current"`
			Progress float32 `json:"progress"`
		}
		type log struct {
			Message string `json:"message"`
		}

		installResponse := func(e modelsindex.StreamMessage) {
			switch e.GetType() {
			case modelsindex.InfoType:
				sseStream.Send(render.SSEEvent{Type: "message", Data: log{Message: e.GetData()}})
			case modelsindex.ProgressType:
				var progressValue float32
				if e.GetProgress().Total > 0 {
					progressValue = float32(e.GetProgress().Current) / float32(e.GetProgress().Total) * 100
				}
				sseStream.Send(render.SSEEvent{Type: "progress", Data: &progress{Name: model.ID, Current: e.GetProgress().Current, Total: e.GetProgress().Total, Progress: progressValue}})

			case modelsindex.ErrorType:
				sseStream.Send(render.SSEEvent{Type: "error", Data: e.GetError()})
			case modelsindex.DoneType:
				sseStream.Send(render.SSEEvent{Type: "done", Data: e.GetDone()})
			}
		}

		err = modelsIndex.Download(r.Context(), dockerClient.Client(), *model, plat, installResponse)
		if err != nil {
			if errors.Is(err, modelsindex.ErrInsufficientStorage) {
				sseStream.SendError(render.SSEErrorData{
					Code:    "insufficient_storage",
					Message: "insufficient disk space to install model",
				})
				return
			}
			sseStream.SendError(render.SSEErrorData{
				Code:    render.InternalServiceErr,
				Message: err.Error(),
			})
		}
	}
}
