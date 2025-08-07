package handlers

import (
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/docker/cli/cli/command"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/pkg/render"
)

func HandleAppLogs(dockerClient command.Cli) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := orchestrator.NewIDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid id"})
			return
		}

		app, err := app.Load(id.ToPath().String())
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", id.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}

		queryParams := r.URL.Query()

		showAppLogs, showServicesLogs := true, false
		if filter := queryParams.Get("filter"); filter != "" {
			filters := strings.Split(strings.TrimSpace(filter), ",")
			showServicesLogs = slices.Contains(filters, "services")
			showAppLogs = slices.Contains(filters, "app")
		}

		var tail *uint64
		if tailStr := queryParams.Get("tail"); tailStr != "" {
			tailParsed, err := strconv.ParseUint(tailStr, 10, 64)
			if err != nil {
				slog.Error("Unable to parse tail", slog.String("error", err.Error()), slog.String("tail", tailStr))
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "invalid tail value"})
				return
			}
			tail = &tailParsed
		}

		// If the follow query param is set, the default is true
		follow := !queryParams.Has("nofollow")

		appLogsRequest := orchestrator.AppLogsRequest{
			ShowAppLogs:      showAppLogs,
			ShowServicesLogs: showServicesLogs,
			Tail:             tail,
			Follow:           follow,
		}

		sseStream, err := render.NewSSEStream(r.Context(), w)
		if err != nil {
			slog.Error("Unable to create SSE stream", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to create SSE stream"})
			return
		}
		defer sseStream.Close()

		type log struct {
			ID      string `json:"id"`
			BrickID string `json:"brick_id,omitempty"`
			Message string `json:"message"`
		}
		messagesIter, err := orchestrator.AppLogs(r.Context(), app, appLogsRequest, dockerClient)
		if err != nil {
			sseStream.SendError(render.SSEErrorData{
				Code:    render.InternalServiceErr,
				Message: "failed to start the app",
			})
			return
		}
		for item := range messagesIter {
			sseStream.Send(render.SSEEvent{Type: "message", Data: log{
				ID:      item.Name,
				Message: item.Content,
				BrickID: item.BrickName,
			}})
		}
	}
}
