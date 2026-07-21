// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/render"
)

func HandleExamples(cfg config.Configuration, idProvider *app.IDProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		examples, err := orchestrator.GetExamples(cfg, idProvider)
		if err != nil {
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: err.Error()})
			return
		}
		render.EncodeResponse(w, http.StatusOK, examples)
	}
}
