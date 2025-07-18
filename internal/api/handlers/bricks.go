package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/pkg/render"
)

func HandleBrickList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := orchestrator.BricksList()
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to retrieve brick list"})

			return
		}
		render.EncodeResponse(w, http.StatusOK, res)
	}
}

func HandleAppBrickInstancesList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		appId, err := orchestrator.NewIDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid app id"})
			return
		}
		appPath := appId.ToPath()

		app, err := app.Load(appPath.String())
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", appId.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}

		res, err := orchestrator.AppBrickInstancesList(&app)
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			details := fmt.Sprintf("unable to find brick list for app %q", appId)
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: details})
			return
		}
		render.EncodeResponse(w, http.StatusOK, res)
	}
}

func HandleAppBrickInstanceDetails() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appId, err := orchestrator.NewIDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid app id"})
			return
		}
		appPath := appId.ToPath()

		app, err := app.Load(appPath.String())
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", appId.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}

		brickID := r.PathValue("brickID")
		if brickID == "" {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "brickID must be set"})
			return
		}

		res, err := orchestrator.AppBrickInstanceDetails(&app, brickID)
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to obtain brick details"})
			return
		}
		render.EncodeResponse(w, http.StatusOK, res)
	}
}

func HandleBrickCreate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appId, err := orchestrator.NewIDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid app id"})
			return
		}
		appPath := appId.ToPath()

		app, err := app.Load(appPath.String())
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", appId.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}

		id := r.PathValue("brickID")
		if id == "" {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "brickID must be set"})
			return
		}

		var req orchestrator.BrickCreateUpdateRequest

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Error("Failed to decode request body", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "invalid request body"})
			return
		}

		req.ID = id

		err = orchestrator.BrickCreate(req, app)
		if err != nil {
			// TODO: handle specific errors
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "error while creating or updating brick"})
			return
		}
		render.EncodeResponse(w, http.StatusOK, nil)
	}
}

func HandleBrickDetails() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("brickID")
		if id == "" {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "id must be set"})
			return
		}
		res, err := orchestrator.BricksDetails(id)
		if err != nil {
			if errors.Is(err, orchestrator.ErrBrickNotFound) {
				details := fmt.Sprintf("brick with id %q not found", id)
				render.EncodeResponse(w, http.StatusNotFound, models.ErrorResponse{Details: details})
				return
			}
			slog.Error("bricks details failed", slog.String("error", err.Error()))
			details := fmt.Sprintf("error getting brick details for id %q", id)
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: details})
			return
		}
		render.EncodeResponse(w, http.StatusOK, res)
	}
}

func HandleBrickUpdates() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appId, err := orchestrator.NewIDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid app id"})
			return
		}
		appPath := appId.ToPath()

		app, err := app.Load(appPath.String())
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", appId.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}

		id := r.PathValue("brickID")
		if id == "" {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "brickID must be set"})
			return
		}

		var req orchestrator.BrickCreateUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Error("Failed to decode request body", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "invalid request body"})
			return
		}

		req.ID = id
		err = orchestrator.BrickUpdate(req, app)
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to update the brick"})

			return
		}

		// TODO decide what we need to return
		render.EncodeResponse(w, http.StatusOK, nil)
	}
}

func HandleBrickPartialUpdates() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("brickID")
		if id == "" {
			render.EncodeResponse(w, http.StatusBadRequest, "id must be set")
			return
		}

		res, err := orchestrator.BricksDetails(id)
		if err != nil {
			return
		}
		render.EncodeResponse(w, http.StatusOK, res)
	}
}

func HandleBrickDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appId, err := orchestrator.NewIDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid app id"})
			return
		}
		appPath := appId.ToPath()

		app, err := app.Load(appPath.String())
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", appId.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}

		id := r.PathValue("brickID")
		log.Printf("DEBUG: Received brickID: '%s'", id)
		if id == "" {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "brickID must be set"})
			return
		}
		err = orchestrator.BrickDelete(id, &app)
		if err != nil {
			switch {
			case errors.Is(err, orchestrator.ErrBrickNotFound):
				details := fmt.Sprintf("brick not found for id %q", id)
				render.EncodeResponse(w, http.StatusNotFound, models.ErrorResponse{Details: details})

			case errors.Is(err, orchestrator.ErrCannotSave):
				log.Printf("Internal error saving brick instance %s: %v", id, err)
				render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to delete the app"})

			default:
				log.Printf("Unexpected error deleting brick %s: %v", id, err)
				render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "A server error occurred while finalizing the deletion."})
			}
			return
		}

		render.EncodeResponse(w, http.StatusOK, nil)
	}
}
