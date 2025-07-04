package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/pkg/render"

	dockerClient "github.com/docker/docker/client"
)

func HandleAppDetails(dockerClient *dockerClient.Client) HandlerAppFunc {
	return func(w http.ResponseWriter, r *http.Request, id orchestrator.ID) {
		if id == "" {
			render.EncodeResponse(w, http.StatusPreconditionFailed, "id must be set")
			return
		}
		appPath := id.ToPath()

		app, err := app.Load(appPath.String())
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", string(id)))
			render.EncodeResponse(w, http.StatusInternalServerError, "unable to find the app")
			return
		}

		res, err := orchestrator.AppDetails(r.Context(), dockerClient, app)
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, "unable to find the app")
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

func HandleAppDetailsEdits() HandlerAppFunc {
	return func(w http.ResponseWriter, r *http.Request, id orchestrator.ID) {
		if id == "" {
			render.EncodeResponse(w, http.StatusPreconditionFailed, "id must be set")
			return
		}
		if id.IsExample() {
			render.EncodeResponse(w, http.StatusBadRequest, "cannot patch example")
			return
		}

		appPath := id.ToPath()

		app, err := app.Load(appPath.String())
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", string(id)))
			render.EncodeResponse(w, http.StatusInternalServerError, "unable to find the app")
			return
		}

		var editRequest EditRequest
		if err := json.NewDecoder(r.Body).Decode(&editRequest); err != nil {
			slog.Error("Unable to decode the request body", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, "invalid request body")
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
			render.EncodeResponse(w, http.StatusInternalServerError, "unable to edit the app")
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
