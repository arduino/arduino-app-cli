// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/render"
)

// loadEditableApp is what every route that changes an app loads it with: it answers the
// request itself when the app cannot be changed, so a handler only gets the token that
// the orchestrator and the brick service take.
func loadEditableApp(w http.ResponseWriter, id appid.ID) (app.Editable, bool) {
	a, err := app.Load(id.ToPath())
	if err != nil {
		slog.Error("Unable to load the app", slog.String("error", err.Error()), slog.String("path", id.String()))
		render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
		return app.Editable{}, false
	}
	editable, err := a.Edit()
	if err != nil {
		render.EncodeResponse(w, http.StatusForbidden, models.ErrorResponse{Details: "cannot alter a release"})
		return app.Editable{}, false
	}
	return editable, true
}
