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

	"github.com/docker/cli/cli/command"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/render"
)

func HandlerAppStatus(
	dockerCli command.Cli,
	idProvider *app.IDProvider,
	bricksIndex *bricksindex.BricksIndex,
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

		result, err := orchestrator.ListApps(r.Context(), dockerCli, orchestrator.ListAppRequest{ShowExamples: true, ShowApps: true}, idProvider, bricksIndex, cfg)
		if err != nil {
			sseStream.SendError(render.SSEErrorData{Code: render.InternalServiceErr, Message: err.Error()})
		}
		for _, app := range result.Apps {
			if app.Status != "" {
				sseStream.Send(render.SSEEvent{Type: "app", Data: app})
			}
		}

		for appStatus, err := range orchestrator.AppStatusEvents(r.Context(), cfg, dockerCli, idProvider) {
			if err != nil {
				sseStream.SendError(render.SSEErrorData{Code: render.InternalServiceErr, Message: err.Error()})
				continue
			}
			sseStream.Send(render.SSEEvent{Type: "app", Data: appStatus})
		}
	}
}
