// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

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
