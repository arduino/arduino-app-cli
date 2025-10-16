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

	"github.com/docker/cli/cli/command"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/render"
	"github.com/arduino/arduino-app-cli/internal/store"
)

func HandleAppStart(
	dockerCli command.Cli,
	provisioner *orchestrator.Provision,
	modelsIndex *modelsindex.ModelsIndex,
	bricksIndex *bricksindex.BricksIndex,
	idProvider *app.IDProvider,
	cfg config.Configuration,
	staticStore *store.StaticStore,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid id"})
			return
		}

		app, err := app.Load(id.ToPath().String())
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", id.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}

		sseStream, err := render.NewSSEStream(r.Context(), w)
		if err != nil {
			slog.Error("Unable to create SSE stream", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to create SSE stream"})
			return
		}
		defer sseStream.Close()

		type progress struct {
			Name     string  `json:"name"`
			Progress float32 `json:"progress"`
		}
		type log struct {
			Message string `json:"message"`
		}
		for item := range orchestrator.StartApp(r.Context(), dockerCli, provisioner, modelsIndex, bricksIndex, app, cfg, staticStore) {
			switch item.GetType() {
			case orchestrator.ProgressType:
				sseStream.Send(render.SSEEvent{Type: "progress", Data: progress(*item.GetProgress())})
			case orchestrator.InfoType:
				sseStream.Send(render.SSEEvent{Type: "message", Data: log{Message: item.GetData()}})
			case orchestrator.ErrorType:
				sseStream.SendError(render.SSEErrorData{
					Code:    render.InternalServiceErr,
					Message: item.GetError().Error(),
				})
			}
		}
	}
}
