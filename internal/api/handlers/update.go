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
