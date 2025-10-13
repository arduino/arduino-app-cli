package handlers

import (
	"log/slog"
	"net/http"
	"time"

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

		// TODO: maybe we should limit the size of this cache?
		stateCache := make(map[string]orchestrator.Status)

		for {
			apps, err := orchestrator.AppStatus(r.Context(), cfg, dockerCli.Client(), idProvider)
			if err != nil {
				slog.Error("Unable to get apps status", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to get apps status"})
				return
			}
			for _, a := range apps {
				status, exist := stateCache[a.ID.String()]
				if !exist || status != a.Status {
					sseStream.Send(render.SSEEvent{Type: "app", Data: a})
				}

				stateCache[a.ID.String()] = a.Status
			}

			select {
			case <-r.Context().Done():
				return
			case <-time.After(1 * time.Second):
			}
		}
	}
}
