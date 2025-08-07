package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/pkg/render"

	"github.com/docker/cli/cli/command"
)

type CloneRequest struct {
	Name *string `json:"name" description:"application name" example:"My Awesome App"`
	Icon *string `json:"icon" description:"application icon"`
}

func HandleAppClone(dockerClient command.Cli) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := orchestrator.NewIDFromBase64(r.PathValue("appID"))
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
		})
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
			if errors.Is(err, orchestrator.ErrInvalidApp) {
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
