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
	"go.bug.st/f"

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
		var refresh bool
		if refreshRaw := r.URL.Query().Get("refresh"); refreshRaw != "" {
			var err error
			refresh, err = strconv.ParseBool(refreshRaw)
			if err != nil {
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "invalid refresh value"})
				return
			}
		}
		var brickFilter []string
		if bricks := strings.TrimSpace(r.URL.Query().Get("bricks")); bricks != "" {
			brickFilter = strings.Split(bricks, ",")
		}
		list, err := orchestrator.AIModelsList(r.Context(), orchestrator.AIModelsListRequest{
			FilterByBrickID: brickFilter,
			Refresh:         refresh,
		}, modelsIndex)
		if err != nil {
			// Without the listing, every model would report the status its declaration
			// carries - not-installed - so a partial answer would be a wrong one.
			slog.Error("cannot get models info", "err", err)
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{
				Details: "cannot determine which models are installed: " + err.Error(),
			})
			return
		}
		render.EncodeResponse(w, http.StatusOK, models.AIModelsListResult{
			Models: f.Map(list, models.NewAIModelItem),
		})
	}
}

// modelIDFromPath reads the model id a path names, so the plain id exists only below
// this line.
func modelIDFromPath(r *http.Request) (string, error) {
	return models.DecodeModelID(r.PathValue("modelID"))
}

func HandlerModelByID(modelsIndex *modelsindex.ModelsIndex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := modelIDFromPath(r)
		if err != nil {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: err.Error()})
			return
		}
		model, found, err := orchestrator.AIModelDetails(r.Context(), modelsIndex, id)
		if err != nil {
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: err.Error()})
			return
		}
		if !found {
			details := fmt.Sprintf("models with id %q not found", id)
			render.EncodeResponse(w, http.StatusNotFound, models.ErrorResponse{Details: details})
			return
		}
		render.EncodeResponse(w, http.StatusOK, models.NewAIModelItem(model))
	}
}

func HandlerDeleteModelByID(dockerClient command.Cli, cfg config.Configuration, modelsIndex *modelsindex.ModelsIndex, bricksIndex *bricksindex.BricksIndex, idProvider *appid.Provider, platform platform.Platform) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := modelIDFromPath(r)
		if err != nil {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: err.Error()})
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
			case errors.Is(err, modelsindex.ErrInstallInProgress):
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
			case errors.Is(err, modelsindex.ErrInstallInProgress):
				slog.Warn("edge impulse install already in progress", "project", projectID, "err", err)
				render.EncodeResponse(w, http.StatusConflict, models.ErrorResponse{Details: "an install of this Edge Impulse model is already in progress"})
				return
			default:
				slog.Error("unable to install Edge Impulse model", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to install Edge Impulse model: " + err.Error()})
				return
			}
		}

		// FIXME: read the installed model using the modelindex.getModelByID
		render.EncodeResponse(w, http.StatusOK, models.NewAIModelItem(eiModel))
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
		id, err := modelIDFromPath(r)
		if err != nil {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: err.Error()})
			return
		}

		unlock, res, err := modelsIndex.PrecheckInstall(r.Context(), dockerClient, id, plat)
		if err != nil {
			writeModelError(w, err) // 404 unknown / 409 in progress / 500 / 507
			return
		}
		defer unlock()

		sseStream, err := render.NewSSEStream(r.Context(), w)
		if err != nil {
			slog.Error("unable to create SSE stream", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to create SSE stream"})
			return
		}
		defer sseStream.Close()

		if res.Installed {
			// D1: nothing to download; answer with the model as the listing reports it.
			model, err := modelsIndex.NewLookup().ByID(r.Context(), id)
			if err != nil || model == nil {
				(&downloadStream{sse: sseStream}).sendError(fmt.Errorf("model %q is installed but not listed: %w", id, err))
				return
			}
			sseStream.Send(render.SSEEvent{Type: "done", Data: models.NewAIModelItem(*model)})
			return
		}

		stream := &downloadStream{sse: sseStream}
		installed, err := orchestrator.AIModelInstall(r.Context(), dockerClient, modelsIndex, plat, id, stream.publish)
		if err != nil {
			stream.sendError(err)
			return
		}
		sseStream.Send(render.SSEEvent{Type: "done", Data: models.NewAIModelItem(installed)})
	}
}

type DownloadModelRequest struct {
	ModelURL  string `json:"model_url" example:"https://huggingface.co/unsloth/SmolLM2-135M-Instruct-GGUF/resolve/main/SmolLM2-135M-Instruct-Q4_K_M.gguf" required:"true"`
	MmprojURL string `json:"mmproj_url" example:"https://huggingface.co/unsloth/SmolLM2-135M-Instruct-GGUF/resolve/main/mmproj-F16.gguf"`
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

		unlock, res, err := modelsIndex.PrecheckDownload(r.Context(), dockerClient.Client(), modelURL, strings.TrimSpace(req.MmprojURL), plat)
		if err != nil {
			writeModelError(w, err) // 400/403/404/409/410/422/500/502/507
			return
		}
		defer unlock()
		if res.Installed {
			// D1: a user download that is already there is a conflict, not a no-op.
			render.EncodeResponse(w, http.StatusConflict, models.ErrorResponse{Details: "model already installed"})
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
		installed, err := orchestrator.AIModelDownload(r.Context(), dockerClient, modelsIndex, plat, modelURL, strings.TrimSpace(req.MmprojURL), stream.publish)
		if err != nil {
			stream.sendError(err)
			return
		}
		sseStream.Send(render.SSEEvent{Type: "done", Data: models.NewAIModelItem(installed)})
	}
}

// sseSender is the part of render.SSEStream a download needs. An interface so a test can
// read the events back without an http.ResponseWriter and the stream's goroutine.
type sseSender interface {
	Send(event render.SSEEvent)
	SendError(event render.SSEErrorData)
}

// downloadStream sends a handler's download events as SSE. After the stream opens, a
// failure is an event, not an HTTP status.
type downloadStream struct {
	sse sseSender
}

func (d *downloadStream) publish(e modelsindex.StreamMessage) {
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
		_, code := modelErrorStatus(modelsindex.ClassifyHandlerError(e.GetError()))
		d.sse.SendError(render.SSEErrorData{Code: render.SSEErrCode(code), Message: e.GetError()})
	case modelsindex.DoneType:
		d.sse.Send(render.SSEEvent{Type: "message", Data: sseLog{Message: e.GetDone()}})
	}
}

func (d *downloadStream) sendError(err error) {
	if errors.Is(err, modelsindex.ErrDownloadReported) {
		// The handler's own error event went out through publish.
		slog.Error("download reported an error", "err", err)
		return
	}
	_, code := modelErrorStatus(err)
	d.sse.SendError(render.SSEErrorData{Code: render.SSEErrCode(code), Message: err.Error()})
}

// modelErrors gives each model failure its HTTP status and a stable code the UI maps to
// its own message. Order matters: the first match wins.
var modelErrors = []struct {
	err    error
	status int
	code   string
}{
	{modelsindex.ErrUnknownModel, http.StatusNotFound, "model_unknown"},
	{modelsindex.ErrInstallInProgress, http.StatusConflict, "install_in_progress"},
	{modelsindex.ErrInsufficientStorage, http.StatusInsufficientStorage, "insufficient_storage"},
	{modelsindex.ErrBadModelURL, http.StatusBadRequest, "invalid_model_url"},
	{modelsindex.ErrModelNotFound, http.StatusNotFound, "model_not_found"},
	{modelsindex.ErrModelForbidden, http.StatusForbidden, "model_private_or_gated"},
	{modelsindex.ErrModelGone, http.StatusGone, "model_disabled"},
	{modelsindex.ErrUnsupportedModel, http.StatusUnprocessableEntity, "model_incompatible"},
	{modelsindex.ErrModelTooLarge, http.StatusUnprocessableEntity, "model_too_large"},
	{modelsindex.ErrNoCompatibleFile, http.StatusUnprocessableEntity, "no_compatible_file"},
	{modelsindex.ErrHubUnreachable, http.StatusBadGateway, "hub_unreachable"},
}

func modelErrorStatus(err error) (int, string) {
	for _, e := range modelErrors {
		if errors.Is(err, e.err) {
			return e.status, e.code
		}
	}
	return http.StatusInternalServerError, string(render.InternalServiceErr)
}

func writeModelError(w http.ResponseWriter, err error) {
	status, code := modelErrorStatus(err)
	render.EncodeResponse(w, status, models.ErrorResponse{Code: code, Details: err.Error()})
}
