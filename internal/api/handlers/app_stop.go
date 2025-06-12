package handlers

import (
	"log/slog"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/pkg/render"

	dockerClient "github.com/docker/docker/client"
)

func HandleAppStop(dockerClient *dockerClient.Client) HandlerAppFunc {
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

		app, err := app.Load(appPath.String())
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", string(id)))
			render.EncodeResponse(w, http.StatusInternalServerError, "unable to find the app")
			return
		}

		sseStream, err := render.NewSSEStream(r.Context(), w)
		if err != nil {
			slog.Error("Unable to create SSE stream", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, "unable to create SSE stream")
			return
		}
		defer sseStream.Close()

		type progress struct {
			Progress float32 `json:"progress"`
		}
		type log struct {
			Message string `json:"message"`
		}
		for item := range orchestrator.StopApp(r.Context(), app) {
			switch item.GetType() {
			case orchestrator.ProgressType:
				sseStream.Send(render.SSEEvent{Type: "progress", Data: progress{Progress: item.GetProgress().Progress}})
			case orchestrator.InfoType:
				sseStream.Send(render.SSEEvent{Type: "message", Data: log{Message: item.GetData()}})
			case orchestrator.ErrorType:
				sseStream.SendError(render.SSEErrorData{
					Code:    render.InternalServiceErr,
					Message: item.GetError().Error(),
				})
			}
		}
	}
}
