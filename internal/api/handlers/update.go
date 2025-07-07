package handlers

import (
	"errors"
	"net/http"
	"strings"

	"log/slog"

	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/apt"
	"github.com/arduino/arduino-app-cli/pkg/render"
)

var matchArduinoPackage = func(p apt.UpgradablePackage) bool {
	return strings.HasPrefix(p.Name, "arduino-")
}

var matchAllPackages = func(p apt.UpgradablePackage) bool {
	return true
}

func HandleCheckUpgradable(aptClient *apt.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		queryParams := r.URL.Query()

		onlyArduinoPackages := false
		if val := queryParams.Get("only-arduino"); val != "" {
			onlyArduinoPackages = strings.ToLower(val) == "true"
		}

		filterFunc := matchAllPackages
		if onlyArduinoPackages {
			filterFunc = matchArduinoPackage
		}

		pkgs, err := aptClient.ListUpgradablePackages(r.Context(), filterFunc)
		if err != nil {
			if errors.Is(err, apt.ErrOperationAlreadyInProgress) {
				render.EncodeResponse(w, http.StatusConflict, err.Error())
				return
			}
			render.EncodeResponse(w, http.StatusBadRequest, "Error checking for upgradable packages: "+err.Error())
			return
		}

		if len(pkgs) == 0 {
			render.EncodeResponse(w, http.StatusNoContent, "System is up to date, no upgradable packages found")
			return
		}

		render.EncodeResponse(w, http.StatusOK, UpdateCheckResult{
			Packages: pkgs,
		})
	}
}

type UpdateCheckResult struct {
	Packages []apt.UpgradablePackage `json:"packages"`
}

func HandleUpdateApply(aptClient *apt.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		queryParams := r.URL.Query()
		onlyArduinoPackages := false
		if val := queryParams.Get("only-arduino"); val != "" {
			onlyArduinoPackages = strings.ToLower(val) == "true"
		}

		filterFunc := matchAllPackages
		if onlyArduinoPackages {
			filterFunc = matchArduinoPackage
		}

		pkgs, err := aptClient.ListUpgradablePackages(r.Context(), filterFunc)
		if err != nil {
			if errors.Is(err, apt.ErrOperationAlreadyInProgress) {
				render.EncodeResponse(w, http.StatusConflict, err.Error())
				return
			}
			slog.Error("Unable to get upgradable packages", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, "Error checking for upgradable packages")
			return
		}

		if len(pkgs) == 0 {
			render.EncodeResponse(w, http.StatusNoContent, "System is up to date, no upgradable packages found")
			return
		}

		names := f.Map(pkgs, func(p apt.UpgradablePackage) string {
			return p.Name
		})

		err = aptClient.UpgradePackages(names)
		if err != nil {
			if errors.Is(err, apt.ErrOperationAlreadyInProgress) {
				render.EncodeResponse(w, http.StatusConflict, err.Error())
				return
			}
			render.EncodeResponse(w, http.StatusInternalServerError, "Error upgrading packages")
			return
		}

		render.EncodeResponse(w, http.StatusAccepted, "Upgrade started")
	}
}

func HandleUpdateEvents(aptClient *apt.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sseStream, err := render.NewSSEStream(r.Context(), w)
		if err != nil {
			slog.Error("Unable to create SSE stream", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, "unable to create SSE stream")
			return
		}
		defer sseStream.Close()

		ch := aptClient.Subscribe()
		defer aptClient.Unsubscribe(ch)

		for {
			select {
			case event, ok := <-ch:
				if !ok {
					slog.Info("APT event channel closed, stopping SSE stream")
					return
				}
				if event.Type == apt.ErrorEvent {
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
