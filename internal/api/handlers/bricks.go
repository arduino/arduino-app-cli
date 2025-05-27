package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/pkg/render"
)

func HandleBrickList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := orchestrator.BricksList()
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, "unable to find the app")
			return
		}
		render.EncodeResponse(w, http.StatusOK, res)
	}
}

func HandleBrickDetails() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			render.EncodeResponse(w, http.StatusBadRequest, "id must be set")
			return
		}

		res, err := orchestrator.BricksDetails(id)
		if err != nil {
			if errors.Is(err, orchestrator.ErrBrickNotFound) {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			slog.Error("bricks details failed", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, "error getting brick details")
			return
		}
		render.EncodeResponse(w, http.StatusOK, res)
	}
}
