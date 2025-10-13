package handlers

import (
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/render"
)

func HandleConfig(cfg config.Configuration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := orchestrator.GetOrchestratorConfig(cfg)
		render.EncodeResponse(w, http.StatusOK, cfg)
	}
}
