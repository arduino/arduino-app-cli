package handlers

import (
	"errors"
	"net/http"
	"strings"

	"log/slog"

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

		render.EncodeResponse(w, http.StatusOK, UpdateCheckResult{
			Packages: pkgs,
		})
	}
}

type UpdateCheckResult struct {
	Packages []orchestrator.UpgradablePackage `json:"packages"`
}

func HandleUpgrade() http.HandlerFunc {
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

		sseStream, err := render.NewSSEStream(r.Context(), w)
		if err != nil {
			slog.Error("Unable to create SSE stream", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, "unable to create SSE stream")
			return
		}
		defer sseStream.Close()

		pkgs, err := orchestrator.GetUpgradablePackages(r.Context(), filterFunc)
		if err != nil {
			slog.Error("Unable to get arduino upgradable packages", slog.String("error", err.Error()))
			sseStream.Send(render.SSEEvent{
				Data: "Error checking for upgradable packages: ",
			})
			return
		}

		iter, err := orchestrator.RunUpgradeCommand(r.Context(), pkgs)
		if err != nil {
			if errors.Is(err, orchestrator.ErrNoUpgradablePackages) {
				sseStream.Send(render.SSEEvent{
					Data: "Already up to date.",
				})
				return
			}

			slog.Error("Error running upgrade command", slog.String("error", err.Error()))
			sseStream.SendError(render.SSEErrorData{
				Code:    render.InternalServiceErr,
				Message: "failed to upgrade the packages",
			})
			return
		}

		for item := range iter {
			sseStream.Send(render.SSEEvent{Data: item})
		}

		sseStream.Send(render.SSEEvent{Type: "restarting", Data: "Upgrade completed. Restarting ..."})

		err = orchestrator.RestartServices(r.Context())
		if err != nil {
			slog.Error("Error restarting services", slog.String("error", err.Error()))
			sseStream.SendError(render.SSEErrorData{
				Code:    render.InternalServiceErr,
				Message: "failed to restart services",
			})
			return
		}
	}
}
