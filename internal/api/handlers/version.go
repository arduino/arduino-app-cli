package handlers

import (
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/render"
)

type VersionResponse struct {
	Version string `json:"version"`
}

func HandlerVersion(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		version := VersionResponse{Version: version}
		render.EncodeResponse(w, http.StatusOK, version)
	}
}
