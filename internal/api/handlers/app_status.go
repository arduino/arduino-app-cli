// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

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
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-app-cli/internal/render"
)

func HandlerAppStatus(
	dockerCli command.Cli,
	idProvider *app.IDProvider,
	bricksIndex *bricksindex.BricksIndex,
	cfg config.Configuration,
	platform platform.Platform,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sseStream, err := render.NewSSEStream(r.Context(), w)
		if err != nil {
			slog.Error("Unable to create SSE stream", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to create SSE stream"})
			return
		}
		defer sseStream.Close()

		result, err := orchestrator.ListApps(r.Context(), dockerCli, orchestrator.ListAppRequest{ShowExamples: true, ShowApps: true}, idProvider, bricksIndex, cfg, platform)
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
