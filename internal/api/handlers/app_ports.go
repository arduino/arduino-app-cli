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
	Port        string `json:"port" example:"80" description:"exposed port	of the app"`
	Source      string `json:"source" example:"brick:data-storage" description:"source of the port, e.g. app or brick:data-storage"`
	ServiceName string `json:"serviceName,omitempty" example:"Web Interface" description:"name of the service if the port is exposed by a brick"`
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
			ServiceName: p.RequiresDisplay,
		})
	}

	return response
}
