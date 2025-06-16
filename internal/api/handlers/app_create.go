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

type CreateAppRequest struct {
	Name   string   `json:"name" description:"application name" example:"My Awesome App" required:"true"`
	Icon   string   `json:"icon" description:"application icon" `
	Bricks []string `json:"bricks,omitempty" description:"application bricks"  example:"[\"core-auth\", \"data-storage\"]"`
}

func HandleAppCreate(dockerClient *dockerClient.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req CreateAppRequest
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
