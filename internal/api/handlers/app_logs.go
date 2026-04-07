// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/render"
)

func HandleAppLogs(
	dockerClient command.Cli,
	idProvider *app.IDProvider,
	bricksIndex *bricksindex.BricksIndex,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid id"})
			return
		}

		app, err := app.Load(id.ToPath())
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
		messagesIter, err := orchestrator.AppLogs(r.Context(), app, appLogsRequest, dockerClient, bricksIndex)
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
