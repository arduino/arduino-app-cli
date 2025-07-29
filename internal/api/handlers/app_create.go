package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/pkg/render"

	dockerClient "github.com/docker/docker/client"
)

type CreateAppRequest struct {
	Name string `json:"name" description:"application name" example:"My Awesome App" required:"true"`
	Icon string `json:"icon" description:"application icon" `
}

func HandleAppCreate(dockerClient *dockerClient.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		defer r.Body.Close()

		queryParams := r.URL.Query()
		skipPythonStr := queryParams.Get("skip-python")
		skipSketchStr := queryParams.Get("skip-sketch")

		skipPython := queryParamsValidator(skipPythonStr)
		skipSketch := queryParamsValidator(skipSketchStr)

		if skipPython && skipSketch {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "cannot skip both python and sketch"})
			return
		}

		var req CreateAppRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Error("unable to decode app create request", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "unable to decode app create request"})
			return
		}

		resp, err := orchestrator.CreateApp(
			r.Context(),
			orchestrator.CreateAppRequest{
				Name:       req.Name,
				Icon:       req.Icon,
				SkipPython: skipPython,
				SkipSketch: skipSketch,
			},
		)
		if err != nil {
			if errors.Is(err, orchestrator.ErrAppAlreadyExists) {
				slog.Error("app already exists", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusConflict, models.ErrorResponse{Details: "app already exists"})
				return
			}
			slog.Error("unable to create app", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to create"})
			return
		}
		render.EncodeResponse(w, http.StatusCreated, resp)
	}
}

func queryParamsValidator(param string) bool {
	if param == "" {
		return false
	}
	b, err := strconv.ParseBool(param)
	if err != nil {
		slog.Warn("query value '%q' for AppCreate non valid: %v\n", param, err)
		return false
	}
	return b
}
