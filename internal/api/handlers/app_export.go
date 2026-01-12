package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/render"
)

func HandleAppExport(
	idProvider *app.IDProvider,
	cfg config.Configuration,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid id"})
			return
		}
		app, err := app.Load(id.ToPath())
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", id.String()))
			if errors.Is(err, os.ErrNotExist) {
				render.EncodeResponse(w, http.StatusNotFound, models.ErrorResponse{Details: "unable to find the app"})
			} else {
				render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to parse the app"})
			}
			return
		}

		zipBytes, fileName, err := orchestrator.ExportApp(r.Context(), app)
		if err != nil {
			slog.Error("failed to export app", slog.String("app_id", id.String()), slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{
				Details: "Failed to generate archive.",
			})
			return
		}

		render.EncodeZipResponse(w, http.StatusOK, zipBytes, fileName)
	}
}
