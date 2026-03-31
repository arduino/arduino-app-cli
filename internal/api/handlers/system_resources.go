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

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
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

		resources, err := orchestrator.SystemResources(ctx, cfg, nil)
		if err != nil {
			sseStream.SendError(render.SSEErrorData{
				Code:    render.InternalServiceErr,
				Message: "failed to obtain the resources",
			})
			return
		}
		for resource := range resources {
			switch res := resource.(type) {
			case *orchestrator.SystemDiskResource:
				sseStream.Send(render.SSEEvent{Type: "disk", Data: res})
			case *orchestrator.SystemCPUResource:
				sseStream.Send(render.SSEEvent{Type: "cpu", Data: res})
			case *orchestrator.SystemMemoryResource:
				sseStream.Send(render.SSEEvent{Type: "mem", Data: res})
			}
		}
	}
}
