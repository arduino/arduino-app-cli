package handlers

import (
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/pkg/render"
)

func HandleConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := orchestrator.GetOrchestratorConfig()
		render.EncodeResponse(w, http.StatusOK, cfg)
	}
}
