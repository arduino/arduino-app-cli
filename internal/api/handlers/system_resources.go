// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/resources"
	"github.com/arduino/arduino-app-cli/internal/render"
)

func HandleSystemResources(cfg config.Configuration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		sseStream, err := render.NewSSEStream(ctx, w)
		if err != nil {
			slog.Error("Unable to create SSE stream", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to create SSE stream"})
			return
		}
		defer sseStream.Close()

		resourcesIt, err := resources.SystemResources(ctx, cfg, nil)
		if err != nil {
			sseStream.SendError(render.SSEErrorData{
				Code:    render.InternalServiceErr,
				Message: "failed to obtain the resources",
			})
			return
		}
		for resource := range resourcesIt {
			switch res := resource.(type) {
			case *resources.SystemDiskResource:
				sseStream.Send(render.SSEEvent{Type: "disk", Data: res})
			case *resources.SystemCPUResource:
				sseStream.Send(render.SSEEvent{Type: "cpu", Data: res})
			case *resources.SystemNPUResource:
				sseStream.Send(render.SSEEvent{Type: "npu", Data: res})
			case *resources.SystemMemoryResource:
				sseStream.Send(render.SSEEvent{Type: "mem", Data: res})
			}
		}
	}
}
