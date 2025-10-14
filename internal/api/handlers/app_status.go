package handlers

import (
	"log/slog"
	"net/http"

	"github.com/docker/cli/cli/command"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/render"
)

func HandlerAppStatus(
	dockerCli command.Cli,
	idProvider *app.IDProvider,
	cfg config.Configuration,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sseStream, err := render.NewSSEStream(r.Context(), w)
		if err != nil {
			slog.Error("Unable to create SSE stream", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to create SSE stream"})
			return
		}
		defer sseStream.Close()

		for appStatus, err := range orchestrator.AppStatusEvents(r.Context(), cfg, dockerCli, idProvider) {
			if err != nil {
				sseStream.SendError(render.SSEErrorData{Code: render.InternalServiceErr, Message: err.Error()})
				continue
			}
			sseStream.Send(render.SSEEvent{Type: "app", Data: appStatus})
		}
	}
}
