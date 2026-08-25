// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

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
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type ResponseLogs struct {
	ID            string `json:"id"`
	ContainerName string `json:"container_name"`
	Message       string `json:"message"`
}

func HandleAppLogs(
	dockerClient command.Cli,
	idProvider *appid.Provider,
	bricksIndex *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid id"})
			return
		}

		app, err := app.Load(id.ToPath())
		if err != nil {
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}

		queryParams := r.URL.Query()

		showAppLogs, showServicesLogs := true, false
		if filter := queryParams.Get("filter"); filter != "" {
			filters := strings.Split(strings.TrimSpace(filter), ",")
			showServicesLogs = slices.Contains(filters, "bricks")
			showAppLogs = slices.Contains(filters, "main")
			if !showAppLogs && !showServicesLogs {
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "invalid filter value"})
				return
			}
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

		messagesIter, err := orchestrator.AppLogs(r.Context(), app, appLogsRequest, dockerClient, bricksIndex, servicesIndex)
		if err != nil {
			sseStream.SendError(render.SSEErrorData{
				Code:    render.InternalServiceErr,
				Message: "failed to start the app",
			})
			return
		}
		for item := range messagesIter {
			switch item.Source {
			case orchestrator.LogSourceMain:
				sseStream.Send(render.SSEEvent{Type: "message", Data: ResponseLogs{
					ID:            "main",
					ContainerName: item.ContainerName,
					Message:       item.Content,
				}})
			case orchestrator.LogSourceBrick:
				sseStream.Send(render.SSEEvent{Type: "message", Data: ResponseLogs{
					ID:            item.BrickID,
					ContainerName: item.ContainerName,
					Message:       item.Content,
				}})
			default:
				slog.Warn("Unknown log source", slog.String("source", string(item.Source)))
			}
		}
	}
}
