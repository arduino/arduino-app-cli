// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type AppPortResponse struct {
	Ports []port `json:"ports" example:"80" description:"exposed port of the app"`
}
type port struct {
	Port       string `json:"port" example:"80" description:"exposed port	of the app"`
	Source     string `json:"source" example:"brick:data-storage" description:"source of the port: app.yaml, a brick ID or the ID of a service required by a brick"`
	SourceType string `json:"sourceType" example:"brick" description:"what the source refers to: app for the app.yaml file, brick for a brick ID, service for the ID of a service required by a brick"`
	Intent     string `json:"intent" example:"webview" description:"what the port is published for: webview to be opened in a browser, user to be reached from outside the board, internal for an implementation detail of the app"`
	// ServiceName never carried a service name: it is the requires_display field of the brick
	// publishing the port. It is kept, unchanged, only to avoid breaking the consumers reading it.
	ServiceName string `json:"serviceName,omitempty" example:"webview" description:"deprecated: use intent instead. Value of the requires_display field of the brick exposing the port"`
}

func HandleAppPorts(
	bricksIndex *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
	idProvider *appid.Provider,
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

		ports, err := orchestrator.AppPorts(app, bricksIndex, servicesIndex)
		if err != nil {
			slog.Error("Unable to find the app ports", slog.String("error", err.Error()), slog.String("path", id.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "Unable to find bricks ports"})
			return
		}

		render.EncodeResponse(w, http.StatusOK, buildAppPortResponse(ports))
	}
}

func buildAppPortResponse(ports []orchestrator.PortInfo) AppPortResponse {
	response := AppPortResponse{
		Ports: make([]port, 0, len(ports)),
	}

	for _, p := range ports {
		response.Ports = append(response.Ports, port{
			Port:        p.Port,
			Source:      p.Source,
			SourceType:  p.SourceType,
			Intent:      p.Intent,
			ServiceName: p.RequiresDisplay,
		})
	}

	return response
}
