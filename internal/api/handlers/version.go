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
	// PythonRunnerVersion is the Python runner image, e.g. ghcr.io/arduino/app-bricks/python-apps-base:0.12.0
	PythonRunnerVersion string `json:"python_runner_version"`
}

func HandlerVersion(version string, pythonRunnerVersion string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		version := VersionResponse{Version: version, PythonRunnerVersion: pythonRunnerVersion}
		render.EncodeResponse(w, http.StatusOK, version)
	}
}
