package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/render"

	"github.com/docker/cli/cli/command"
)

func HandleAppDetails(
	dockerClient command.Cli,
	bricksIndex *bricksindex.BricksIndex,
	idProvider *app.IDProvider,
	cfg config.Configuration,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idProvider.IDFromBase64(r.PathValue("appID"))
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

		res, err := orchestrator.AppDetails(r.Context(), dockerClient, app, bricksIndex, idProvider, cfg)
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

func HandleAppDetailsEdits(
	dockerClient command.Cli,
	bricksIndex *bricksindex.BricksIndex,
	idProvider *app.IDProvider,
	cfg config.Configuration,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid id"})
			return
		}
		appToEdit, err := app.Load(id.ToPath().String())
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", id.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}

		var appEditRequest orchestrator.AppEditRequest
		var editRequest EditRequest

		if err := json.NewDecoder(r.Body).Decode(&editRequest); err != nil {
			slog.Error("Unable to decode the request body", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "invalid request"})
			return
		}
		if id.IsExample() {
			if editRequest.Description != nil || editRequest.Icon != nil || editRequest.Name != nil {
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "you can patch just the default field for example apps"})
				return
			}
			appEditRequest = orchestrator.AppEditRequest{
				Default: editRequest.Default,
			}
		} else {
			appEditRequest = orchestrator.AppEditRequest{
				Default:     editRequest.Default,
				Name:        editRequest.Name,
				Icon:        editRequest.Icon,
				Description: editRequest.Description,
			}
		}
		err = orchestrator.EditApp(appEditRequest, &appToEdit, cfg)
		if err != nil {
			switch {
			case errors.Is(err, app.ErrInvalidApp):
				slog.Error("Unable to edit the app 1", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: err.Error()})
			case errors.Is(err, orchestrator.ErrAppAlreadyExists):
				slog.Error("The name is already in use.", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{
					Details: fmt.Sprintf("the name %q is already in use", *editRequest.Name),
				})
			default:
				slog.Error("Unable to edit the app ", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to edit the app"})
			}
			return
		}

		res, err := orchestrator.AppDetails(r.Context(), dockerClient, appToEdit, bricksIndex, idProvider, cfg)
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}
		render.EncodeResponse(w, http.StatusOK, res)
	}
}
