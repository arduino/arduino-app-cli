package handlers

import (
	"log/slog"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/pkg/render"
)

func HandleAppDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := orchestrator.NewIDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, "invalid id")
			return
		}
		if id.IsExample() {
			render.EncodeResponse(w, http.StatusBadRequest, "cannot delete example")
			return
		}

		app, err := app.Load(id.ToPath().String())
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", id.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, "unable to find the app")
			return
		}

		err = orchestrator.DeleteApp(r.Context(), app)
		if err != nil {
			slog.Error("Unable to delete the app", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, "unable to delete the app")
			return
		}
		render.EncodeResponse(w, http.StatusOK, nil)
	}
}
