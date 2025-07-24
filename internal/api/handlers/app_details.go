package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/pkg/render"

	dockerClient "github.com/docker/docker/client"
)

func HandleAppDetails(dockerClient *dockerClient.Client, bricksIndex *bricksindex.BricksIndex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := orchestrator.NewIDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid id"})
			return
		}

		app, err := app.Load(id.ToPath().String())
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", id.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}

		res, err := orchestrator.AppDetails(r.Context(), dockerClient, app, bricksIndex)
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}
		render.EncodeResponse(w, http.StatusOK, res)
	}
}

type EditRequest struct {
	Name        *string `json:"name" example:"My Awesome App" description:"application name"`
	Icon        *string `json:"icon" example:"💻" description:"application icon"`
	Description *string `json:"description" example:"This is my awesome app" description:"application description"`
	Default     *bool   `json:"default"`
}

func HandleAppDetailsEdits(dockerClient *dockerClient.Client, bricksIndex *bricksindex.BricksIndex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := orchestrator.NewIDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid id"})
			return
		}
		if id.IsExample() {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "cannot patch the example"})
			return
		}

		app, err := app.Load(id.ToPath().String())
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", id.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}

		var editRequest EditRequest
		if err := json.NewDecoder(r.Body).Decode(&editRequest); err != nil {
			slog.Error("Unable to decode the request body", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "invalid request"})
			return
		}
		err = orchestrator.EditApp(orchestrator.AppEditRequest{
			Default:     editRequest.Default,
			Name:        editRequest.Name,
			Icon:        editRequest.Icon,
			Description: editRequest.Description,
		}, &app)
		if err != nil {
			slog.Error("Unable to edit the app", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to edit the app"})
			return
		}
		res, err := orchestrator.AppDetails(r.Context(), dockerClient, app, bricksIndex)
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}
		render.EncodeResponse(w, http.StatusOK, res)
	}
}
