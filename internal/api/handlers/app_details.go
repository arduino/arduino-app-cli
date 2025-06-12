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
		appPath, err := id.ToPath()
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, "invalid id")
			return
		}

		app, err := app.Load(appPath.String())
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", string(id)))
			render.EncodeResponse(w, http.StatusInternalServerError, "unable to find the app")
			return
		}

		res, err := orchestrator.AppDetails(r.Context(), app)
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, "unable to find the app")
			return
		}
		render.EncodeResponse(w, http.StatusOK, res)
	}
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

		appPath, err := id.ToPath()
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, "invalid id")
			return
		}

		app, err := app.Load(appPath.String())
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", string(id)))
			render.EncodeResponse(w, http.StatusInternalServerError, "unable to find the app")
			return
		}

		type EditRequest struct {
			Default *bool `json:"default"`
			// The key is brick name, the second map is variable_name -> value.
			Variables *map[string]map[string]string `json:"variables"`
		}

		var editRequest EditRequest
		if err := json.NewDecoder(r.Body).Decode(&editRequest); err != nil {
			slog.Error("Unable to decode the request body", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, "invalid request body")
			return
		}

		err = orchestrator.EditApp(orchestrator.AppEditRequest{
			Default:   editRequest.Default,
			Variables: editRequest.Variables,
		}, &app)
		if err != nil {
			slog.Error("Unable to edit the app", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, "unable to edit the app")
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
