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
	"net/http"
	"strings"

	"log/slog"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/render"
	"github.com/arduino/arduino-app-cli/internal/update"
)

func HandleCheckUpgradable(updater *update.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		queryParams := r.URL.Query()

		onlyArduinoPackages := false
		if val := queryParams.Get("only-arduino"); val != "" {
			onlyArduinoPackages = strings.ToLower(val) == "true"
		}

		filterFunc := update.MatchAllPackages
		if onlyArduinoPackages {
			filterFunc = update.MatchArduinoPackage
		}

		pkgs, err := updater.ListUpgradablePackages(r.Context(), filterFunc)
		if err != nil {
			code := update.GetUpdateErrorCode(err)
			if code == update.OperationInProgressCode {
				render.EncodeResponse(w, http.StatusConflict, models.ErrorResponse{
					Code:    string(code),
					Details: err.Error(),
				})
				return
			}
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{
				Code:    string(code),
				Details: err.Error(),
			})
			return
		}
		if len(pkgs) == 0 {
			render.EncodeResponse(w, http.StatusNoContent, nil)
			return
		}

		render.EncodeResponse(w, http.StatusOK, UpdateCheckResult{Packages: pkgs})
	}
}

type UpdateCheckResult struct {
	Packages []update.UpgradablePackage `json:"updates"`
}

func HandleUpdateApply(updater *update.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		queryParams := r.URL.Query()
		onlyArduinoPackages := false
		if val := queryParams.Get("only-arduino"); val != "" {
			onlyArduinoPackages = strings.ToLower(val) == "true"
		}

		filterFunc := update.MatchAllPackages
		if onlyArduinoPackages {
			filterFunc = update.MatchArduinoPackage
		}

		pkgs, err := updater.ListUpgradablePackages(r.Context(), filterFunc)
		if err != nil {
			code := update.GetUpdateErrorCode(err)
			if code == update.OperationInProgressCode {
				render.EncodeResponse(w, http.StatusConflict, models.ErrorResponse{
					Code:    string(code),
					Details: err.Error(),
				})
				return
			}
			slog.Error("Unable to get upgradable packages", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{
				Code:    string(code),
				Details: err.Error(),
			})
			return
		}
		if len(pkgs) == 0 {
			render.EncodeResponse(w, http.StatusNoContent, nil)
			return
		}

		err = updater.UpgradePackages(r.Context(), pkgs)
		if err != nil {
			code := update.GetUpdateErrorCode(err)
			if code == update.OperationInProgressCode {
				render.EncodeResponse(w, http.StatusConflict, models.ErrorResponse{
					Code:    string(code),
					Details: err.Error(),
				})
				return
			}
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{
				Code:    string(code),
				Details: err.Error(),
			})
			return
		}

		render.EncodeResponse(w, http.StatusAccepted, "Upgrade started")
	}
}

func HandleUpdateEvents(updater *update.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// HOTFIX: app-lab use HEAD requests to check endpoint availability
		// so we need to handle them here by early return without opening SSE stream
		if r.Method == http.MethodHead {
			render.EncodeResponse(w, http.StatusOK, nil)
			return
		}

		sseStream, err := render.NewSSEStream(r.Context(), w)
		if err != nil {
			slog.Error("Unable to create SSE stream", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to create SSE stream"})
			return
		}
		defer sseStream.Close()

		ch := updater.Subscribe()
		defer updater.Unsubscribe(ch)

		for {
			select {
			case event, ok := <-ch:
				if !ok {
					slog.Info("APT event channel closed, stopping SSE stream")
					return
				}
				if event.Type == update.ErrorEvent {
					err := event.GetError()
					code := render.InternalServiceErr
					if c := update.GetUpdateErrorCode(err); c != update.UnknownErrorCode {
						code = render.SSEErrCode(string(c))
					}
					sseStream.SendError(render.SSEErrorData{
						Code:    code,
						Message: err.Error(),
					})
				} else {
					sseStream.Send(render.SSEEvent{
						Type: event.Type.String(),
						Data: event.GetData(),
					})
				}

			case <-r.Context().Done():
				return
			}
		}
	}
}
