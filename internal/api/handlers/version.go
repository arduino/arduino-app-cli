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
	// BricksVersion is the Python runner image used by bricks, e.g. ghcr.io/arduino/app-bricks/python-apps-base:0.12.0
	BricksVersion string `json:"bricks_version"`
}

func HandlerVersion(version string, bricksVersion string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		version := VersionResponse{Version: version, BricksVersion: bricksVersion}
		render.EncodeResponse(w, http.StatusOK, version)
	}
}
