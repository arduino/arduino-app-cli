// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/render"
)

// The intent tells the consumer what to do with a port.
const (
	// webviewIntent marks a port serving the app web UI: the consumer opens it in a browser.
	webviewIntent = "webview"
	// userIntent marks a port the user may want to reach from the host: the consumer forwards it.
	userIntent = "user"
	// internalIntent marks a port published by a service required by a brick: it is an
	// implementation detail of the app, the consumer ignores it.
	internalIntent = "internal"
)

const appPortsSource = "app.yaml"

type AppPortResponse struct {
	Ports []port `json:"ports" example:"80" description:"exposed port of the app"`
}
type port struct {
	Port        string `json:"port" example:"80" description:"exposed port	of the app"`
	Source      string `json:"source" example:"brick:data-storage" description:"source of the port: app.yaml, a brick ID or the ID of a service required by a brick"`
	ServiceName string `json:"serviceName,omitempty" example:"webview" description:"what the consumer should do with the port: webview to open it in a browser, user to forward it, internal to ignore it"`
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

		index := bricksIndex.WithAppBricks(app.LocalBricks)
		brickPorts, err := getBrickPorts(app.Descriptor.Bricks, index)
		if err != nil {
			slog.Error("Unable to find bricks ports", slog.String("error", err.Error()), slog.String("path", id.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "Unable to find bricks ports"})
			return
		}

		services, err := orchestrator.RequiredServices(app.Descriptor.Bricks, index, servicesIndex)
		if err != nil {
			slog.Error("Unable to find service ports", slog.String("error", err.Error()), slog.String("path", id.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "Unable to find service ports"})
			return
		}

		response := buildAppPortResponse(app.Descriptor.Ports, brickPorts, services)

		render.EncodeResponse(w, http.StatusOK, response)
	}
}

// buildAppPortResponse lists the app ports first, then the brick ones in the order the bricks are
// declared by the app, then the service ones: the consumers act on the ports in the order they are
// received, so the response must not depend on a map iteration.
func buildAppPortResponse(appPorts []int, bricks []brickPorts, services []orchestrator.RequiredService) AppPortResponse {
	response := AppPortResponse{
		Ports: make([]port, 0, len(appPorts)+len(bricks)+len(services)),
	}

	for _, p := range appPorts {
		response.Ports = append(response.Ports, port{
			Port:        strconv.Itoa(p),
			Source:      appPortsSource,
			ServiceName: userIntent,
		})
	}

	for _, brick := range bricks {
		for _, p := range brick.ports {
			response.Ports = append(response.Ports, port{
				Port:        p,
				Source:      brick.brickID,
				ServiceName: brick.intent,
			})
		}
	}

	for _, service := range services {
		for _, p := range service.Ports {
			response.Ports = append(response.Ports, port{
				Port:        p,
				Source:      service.ID,
				ServiceName: internalIntent,
			})
		}
	}

	return response
}

// brickPorts are the ports published by a single app brick, and what the consumer should do with
// them.
type brickPorts struct {
	brickID string
	ports   []string
	intent  string
}

func getBrickPorts(bricks []app.Brick, bricksIndex *bricksindex.BricksIndex) ([]brickPorts, error) {
	result := make([]brickPorts, 0, len(bricks))

	for _, brick := range bricks {
		brickData, found := bricksIndex.FindBrickByID(brick.ID)
		if !found {
			return nil, fmt.Errorf("brick %q not found in the index", brick.ID)
		}
		result = append(result, brickPorts{
			brickID: brick.ID,
			ports:   brickData.GetPorts(),
			intent:  brickIntent(brickData.RequiresDisplay),
		})
	}

	return result, nil
}

func brickIntent(requiresDisplay string) string {
	if requiresDisplay == webviewIntent {
		return webviewIntent
	}
	return userIntent
}
