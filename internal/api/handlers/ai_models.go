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
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
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

func HandlerDeleteModelByID(dockerClient command.Cli, cfg config.Configuration, modelsIndex *modelsindex.ModelsIndex, bricksIndex *bricksindex.BricksIndex, idProvider *appid.Provider, platform platform.Platform) http.HandlerFunc {
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

type sseProgress struct {
	Name     string  `json:"name"`
	Total    int64   `json:"total"`
	Current  int64   `json:"current"`
	Progress float32 `json:"progress"`
}

type sseLog struct {
	Message string `json:"message"`
}

// HandleInstallModel installs a model from the internal model list. HandleDownloadModel
// downloads a model that no entry declares.
func HandleInstallModel(dockerClient command.Cli, modelsIndex *modelsindex.ModelsIndex, plat platform.Platform) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.PathValue("modelID"))
		if id == "" {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "model ID must be set"})
			return
		}

		// The declaration alone answers this, so no listing container runs.
		declared, found := modelsIndex.DeclaredByID(id)
		if !found {
			details := fmt.Sprintf("no model with id %q is declared; download a Hugging Face model with POST /v1/models", id)
			render.EncodeResponse(w, http.StatusNotFound, models.ErrorResponse{Details: details})
			return
		}

		sseStream, err := render.NewSSEStream(r.Context(), w)
		if err != nil {
			slog.Error("unable to create SSE stream", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to create SSE stream"})
			return
		}
		defer sseStream.Close()

		if declared.InstalledByDeclaration() {
			// Installed by its declaration, with no handler to run.
			sseStream.Send(render.SSEEvent{Type: "done", Data: orchestrator.NewAIModelItem(*declared)})
			return
		}

		stream := &downloadStream{sse: sseStream}
		if err := modelsIndex.Download(r.Context(), dockerClient.Client(), *declared, plat, stream.publish); err != nil {
			stream.sendError(err)
			return
		}
		if stream.failed {
			return
		}

		// The second result is false only when the declaration is nil.
		installed, _ := installedModel(modelsIndex, declared, stream.downloaded)
		sseStream.Send(render.SSEEvent{Type: "done", Data: orchestrator.NewAIModelItem(installed)})
	}
}

type DownloadModelRequest struct {
	ModelURL  string `json:"model_url" description:"URL of the GGUF model file on Hugging Face" example:"https://huggingface.co/unsloth/SmolLM2-135M-Instruct-GGUF/resolve/main/SmolLM2-135M-Instruct-Q4_K_M.gguf" required:"true"`
	MmprojURL string `json:"model_mmproj_url" description:"URL of the GGUF multimodal projection file on Hugging Face, for a vision model" example:"https://huggingface.co/unsloth/SmolLM2-135M-Instruct-GGUF/resolve/main/mmproj-F16.gguf"`
}

// HandleDownloadModel downloads a model that no models-list.yaml entry declares. The id is
// not an input: the downloader makes it from the file that arrives, and reports it.
func HandleDownloadModel(dockerClient command.Cli, modelsIndex *modelsindex.ModelsIndex, plat platform.Platform) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req DownloadModelRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "unable to decode download model request"})
			return
		}
		modelURL := strings.TrimSpace(req.ModelURL)
		if modelURL == "" {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "model_url must be set"})
			return
		}

		sseStream, err := render.NewSSEStream(r.Context(), w)
		if err != nil {
			slog.Error("unable to create SSE stream", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to create SSE stream"})
			return
		}
		defer sseStream.Close()

		stream := &downloadStream{sse: sseStream}
		err = modelsIndex.DownloadByURL(r.Context(), dockerClient.Client(), modelURL, strings.TrimSpace(req.MmprojURL), plat, stream.publish)
		if err != nil {
			stream.sendError(err)
			return
		}
		if stream.failed {
			return
		}

		installed, ok := installedModel(modelsIndex, nil, stream.downloaded)
		if !ok {
			slog.Error("download named no model", "model_url", modelURL)
			sseStream.SendError(render.SSEErrorData{
				Code:    render.InternalServiceErr,
				Message: "download named no model: a newer models-downloader image is required",
			})
			return
		}
		sseStream.Send(render.SSEEvent{Type: "done", Data: orchestrator.NewAIModelItem(installed)})
	}
}

// downloadStream sends a handler's download events as SSE, and keeps the model that the
// handler names. After the stream opens, a failure is an event, not an HTTP status.
type downloadStream struct {
	sse        *render.SSEStream
	downloaded *modelsindex.DownloadedModel
	failed     bool
}

func (d *downloadStream) publish(e modelsindex.StreamMessage) {
	if m := e.GetModel(); m != nil {
		d.downloaded = m
	}
	switch e.GetType() {
	case modelsindex.InfoType:
		d.sse.Send(render.SSEEvent{Type: "message", Data: sseLog{Message: e.GetData()}})
	case modelsindex.ProgressType:
		p := e.GetProgress()
		var progress float32
		if p.Total > 0 {
			progress = float32(p.Current) / float32(p.Total) * 100
		}
		d.sse.Send(render.SSEEvent{Type: "progress", Data: sseProgress{
			Name: p.Name, Current: p.Current, Total: p.Total, Progress: progress,
		}})
	case modelsindex.ErrorType:
		d.failed = true
		d.sse.Send(render.SSEEvent{Type: "error", Data: e.GetError()})
	case modelsindex.DoneType:
		d.sse.Send(render.SSEEvent{Type: "message", Data: sseLog{Message: e.GetDone()}})
	}
}

func (d *downloadStream) sendError(err error) {
	if errors.Is(err, modelsindex.ErrInsufficientStorage) {
		d.sse.SendError(render.SSEErrorData{Code: "insufficient_storage", Message: "insufficient disk space to install model"})
		return
	}
	d.sse.SendError(render.SSEErrorData{Code: render.InternalServiceErr, Message: err.Error()})
}

func installedModel(modelsIndex *modelsindex.ModelsIndex, declared *modelsindex.AIModel, downloaded *modelsindex.DownloadedModel) (modelsindex.AIModel, bool) {
	if declared == nil {
		if downloaded == nil {
			return modelsindex.AIModel{}, false
		}
		return modelsIndex.InstalledModel(*downloaded), true
	}
	installed := *declared
	installed.Status = modelsindex.InstalledStatus
	if downloaded != nil && downloaded.Size > 0 {
		installed.Size = downloaded.Size
	}
	return installed, true
}
