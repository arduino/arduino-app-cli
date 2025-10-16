// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package handlers

import (
	"log/slog"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/render"
)

func HandleSystemResources() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		sseStream, err := render.NewSSEStream(ctx, w)
		if err != nil {
			slog.Error("Unable to create SSE stream", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to create SSE stream"})
			return
		}
		defer sseStream.Close()

		resources, err := orchestrator.SystemResources(ctx, nil)
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
