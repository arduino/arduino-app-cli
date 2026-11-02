// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"net/http"
	"time"

	"github.com/arduino/arduino-app-cli/internal/render"
)

type VersionResponse struct {
	Version string `json:"version"`
}

func HandlerVersion(version string) http.HandlerFunc {
	// App Lab 0.11.0 and older start a second self-update when this answers right after a restart.
	availableAt := time.Now().Add(20 * time.Second)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "" && time.Now().Before(availableAt) {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		version := VersionResponse{Version: version}
		render.EncodeResponse(w, http.StatusOK, version)
	}
}
