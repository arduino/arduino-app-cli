package handlers

import (
	"log/slog"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/pkg/parser"
	"github.com/arduino/arduino-app-cli/pkg/render"
)

func HandleAppDelete() HandlerAppFunc {
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

		app, err := parser.Load(appPath.String())
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", string(id)))
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
