package handlers

import (
	"errors"
	"net/http"
	"strings"

	"log/slog"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/update"
	"github.com/arduino/arduino-app-cli/pkg/render"
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
			if errors.Is(err, update.ErrOperationAlreadyInProgress) {
				render.EncodeResponse(w, http.StatusConflict, models.ErrorResponse{Details: err.Error()})
				return
			}
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "Error checking for upgradable packages: " + err.Error()})
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
			if errors.Is(err, update.ErrOperationAlreadyInProgress) {
				render.EncodeResponse(w, http.StatusConflict, models.ErrorResponse{Details: err.Error()})
				return
			}
			slog.Error("Unable to get upgradable packages", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "Error checking for upgradable packages"})
			return
		}

		if len(pkgs) == 0 {
			render.EncodeResponse(w, http.StatusNoContent, models.ErrorResponse{Details: "System is up to date, no upgradable packages found"})
			return
		}

		err = updater.UpgradePackages(r.Context(), pkgs)
		if err != nil {
			if errors.Is(err, update.ErrOperationAlreadyInProgress) {
				render.EncodeResponse(w, http.StatusConflict, models.ErrorResponse{Details: err.Error()})
				return
			}
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "Error upgrading packages"})
			return
		}

		render.EncodeResponse(w, http.StatusAccepted, "Upgrade started")
	}
}

func HandleUpdateEvents(updater *update.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
					sseStream.SendError(render.SSEErrorData{
						Code:    render.InternalServiceErr,
						Message: event.Data,
					})
				} else {
					sseStream.Send(render.SSEEvent{
						Type: event.Type.String(),
						Data: event.Data,
					})
				}

			case <-r.Context().Done():
				return
			}
		}
	}
}
