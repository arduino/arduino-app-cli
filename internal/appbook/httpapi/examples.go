// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package httpapi

import (
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/appbook"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/render"
)

func HandleExamples(cfg config.Configuration, brickIndex *bricksindex.BricksIndex, idProvider *appid.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		examples, err := appbook.GetExamples(cfg, brickIndex, idProvider)
		if err != nil {
			render.EncodeResponse(w, http.StatusInternalServerError, render.ErrorResponse{Details: err.Error()})
			return
		}
		render.EncodeResponse(w, http.StatusOK, examples)
	}
}
