package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/pkg/render"
)

func HandleBrickList(
	modelsIndex *modelsindex.ModelsIndex,
	bricksIndex *bricksindex.BricksIndex,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := orchestrator.BricksList(modelsIndex, bricksIndex)
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to retrieve brick list"})

			return
		}
		render.EncodeResponse(w, http.StatusOK, res)
	}
}

func HandleAppBrickInstancesList(bricksIndex *bricksindex.BricksIndex) http.HandlerFunc {
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

		res, err := orchestrator.AppBrickInstancesList(&app, bricksIndex)
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			details := fmt.Sprintf("unable to find brick list for app %q", appId)
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: details})
			return
		}
		render.EncodeResponse(w, http.StatusOK, res)
	}
}

func HandleAppBrickInstanceDetails(bricksIndex *bricksindex.BricksIndex) http.HandlerFunc {
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

		res, err := orchestrator.AppBrickInstanceDetails(&app, bricksIndex, brickID)
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to obtain brick details"})
			return
		}
		render.EncodeResponse(w, http.StatusOK, res)
	}
}

func HandleBrickCreate(
	modelsIndex *modelsindex.ModelsIndex,
	bricksIndex *bricksindex.BricksIndex,
) http.HandlerFunc {
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

		err = orchestrator.BrickCreate(req, modelsIndex, bricksIndex, app)
		if err != nil {
			// TODO: handle specific errors
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "error while creating or updating brick"})
			return
		}
		render.EncodeResponse(w, http.StatusOK, nil)
	}
}

func HandleBrickDetails(
	docsFS fs.FS,
	bricksIndex *bricksindex.BricksIndex,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("brickID")
		if id == "" {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "id must be set"})
			return
		}
		res, err := orchestrator.BricksDetails(docsFS, bricksIndex, id)
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

func HandleBrickUpdates(
	modelsIndex *modelsindex.ModelsIndex,
	bricksIndex *bricksindex.BricksIndex,
) http.HandlerFunc {
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
		err = orchestrator.BrickUpdate(req, modelsIndex, bricksIndex, app)
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to update the brick"})

			return
		}

		// TODO decide what we need to return
		render.EncodeResponse(w, http.StatusOK, nil)
	}
}

func HandleBrickPartialUpdates(docsFS fs.FS, bricksIndex *bricksindex.BricksIndex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("brickID")
		if id == "" {
			render.EncodeResponse(w, http.StatusBadRequest, "id must be set")
			return
		}

		res, err := orchestrator.BricksDetails(docsFS, bricksIndex, id)
		if err != nil {
			return
		}
		render.EncodeResponse(w, http.StatusOK, res)
	}
}

func HandleBrickDelete(bricksIndex *bricksindex.BricksIndex) http.HandlerFunc {
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
		err = orchestrator.BrickDelete(bricksIndex, id, &app)
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
