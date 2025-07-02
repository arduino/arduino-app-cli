package handlers

import (
	"net/http"
	"strings"
	"sync/atomic"

	"log/slog"

	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/pkg/render"
)

var matchArduinoPackage = func(p orchestrator.UpgradablePackage) bool {
	return strings.HasPrefix(p.Name, "arduino-")
}

var matchAllPackages = func(p orchestrator.UpgradablePackage) bool {
	return true
}

func HandleCheckUpgradable() http.HandlerFunc {
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

		pkgs, err := orchestrator.GetUpgradablePackages(r.Context(), filterFunc)
		if err != nil {
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
	Packages []orchestrator.UpgradablePackage `json:"packages"`
}

var inProgress atomic.Bool

func HandleUpdateApply(eventsBroker *UpdateEventsBroker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if an upgrade is already in progress
		if inProgress.Load() {
			render.EncodeResponse(w, http.StatusConflict, "Upgrade already in progress")
			return
		}

		// Set upgrade in progress
		if !inProgress.CompareAndSwap(false, true) {
			render.EncodeResponse(w, http.StatusConflict, "Upgrade already in progress")
			return
		}

		queryParams := r.URL.Query()
		onlyArduinoPackages := false
		if val := queryParams.Get("only-arduino"); val != "" {
			onlyArduinoPackages = strings.ToLower(val) == "true"
		}

		filterFunc := matchAllPackages
		if onlyArduinoPackages {
			filterFunc = matchArduinoPackage
		}

		pkgs, err := orchestrator.GetUpgradablePackages(r.Context(), filterFunc)
		if err != nil {
			slog.Error("Unable to get upgradable packages", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, "Error checking for upgradable packages")
			return
		}

		if len(pkgs) == 0 {
			render.EncodeResponse(w, http.StatusNoContent, "System is up to date, no upgradable packages found")
			return
		}

		go func() {
			defer inProgress.Store(false)

			names := f.Map(pkgs, func(p orchestrator.UpgradablePackage) string {
				return p.Name
			})

			eventsBroker.PublishLog("Upgrading: " + strings.Join(names, ", "))

			iter, err := orchestrator.RunUpgradeCommand(r.Context(), names)
			if err != nil {
				slog.Error("Error running upgrade command", slog.String("error", err.Error()))
				eventsBroker.PublishError(render.SSEErrorData{Message: "failed to upgrade the packages"})
				return
			}

			for item := range iter {
				eventsBroker.PublishLog(item)
			}

			eventsBroker.Restarting()

			err = orchestrator.RestartServices(r.Context())
			if err != nil {
				slog.Error("Error restarting services", slog.String("error", err.Error()))
				eventsBroker.PublishError(render.SSEErrorData{Message: "failed to restart services"})
				return
			}
		}()

		render.EncodeResponse(w, http.StatusAccepted, "Upgrade started")
	}
}

func HandleUpdateEvents(eventsBroker *UpdateEventsBroker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sseStream, err := render.NewSSEStream(r.Context(), w)
		if err != nil {
			slog.Error("Unable to create SSE stream", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, "unable to create SSE stream")
			return
		}
		defer sseStream.Close()

		ch := eventsBroker.Subscribe()
		defer eventsBroker.Unsubscribe(ch)

		for {
			select {
			case event := <-ch:
				sseStream.Send(event)
			case <-r.Context().Done():
				return
			}
		}
	}
}
