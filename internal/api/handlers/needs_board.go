// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/render"
)

// NeedsBoard answers what needs a board, where there is none.
func NeedsBoard() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		render.EncodeResponse(w, http.StatusNotImplemented, models.ErrorResponse{Code: "needs_board", Details: "this needs a board"})
	}
}
