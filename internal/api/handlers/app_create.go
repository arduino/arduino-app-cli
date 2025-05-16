package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/pkg/render"

	dockerClient "github.com/docker/docker/client"
)

func HandleAppCreate(dockerClient *dockerClient.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type CreateRequest struct {
			Name   string   `json:"name"`
			Icon   string   `json:"icon"`
			Bricks []string `json:"bricks"`
		}
		defer r.Body.Close()

		var req CreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Error("unable to decode app create request", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, "unable to decode app create request")
			return
		}

		resp, err := orchestrator.CreateApp(
			r.Context(),
			orchestrator.CreateAppRequest{
				Name:   req.Name,
				Icon:   req.Icon,
				Bricks: req.Bricks,
			},
		)
		if err != nil {
			if errors.Is(err, orchestrator.ErrAppAlreadyExists) {
				slog.Error("app already exists", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusConflict, "app already exists")
				return
			}
			slog.Error("unable to create app", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, "unable to create app")
			return
		}
		render.EncodeResponse(w, http.StatusCreated, resp)
	}
}
