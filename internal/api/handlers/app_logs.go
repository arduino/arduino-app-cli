package handlers

import (
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/pkg/render"

	dockerClient "github.com/docker/docker/client"
)

func HandleAppLogs(dockerClient *dockerClient.Client) http.HandlerFunc {
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

		tail := int64(0)
		if tailStr := queryParams.Get("tail"); tailStr != "" {
			tail, err = strconv.ParseInt(tailStr, 10, 64)
			if err != nil {
				slog.Error("Unable to parse tail", slog.String("error", err.Error()), slog.String("tail", tailStr))
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "invalid tail value"})
				return
			}
			if tail < 0 {
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "invalid tail value, cannot be negative"})
				return
			}
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
		messagesIter, err := orchestrator.AppLogs(r.Context(), app, appLogsRequest)
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
