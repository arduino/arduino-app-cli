package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

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
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}

		includeData := false
		if val := r.URL.Query().Get("include_data"); val != "" {
			var err error
			includeData, err = strconv.ParseBool(val)
			if err != nil {
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{
					Details: "The parameter 'include_data' must be a boolean.",
				})
				return
			}
		}

		zipBytes, fileName, err := orchestrator.ExportApp(r.Context(), app, includeData)
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
