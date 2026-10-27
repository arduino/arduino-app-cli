// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package daemon

import (
	"log/slog"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/api/handlers"
	"github.com/arduino/arduino-app-cli/internal/editor"
)

// withEditor serves the editor service (internal/editor) at /v1/edit, next to the API:
// the one place the daemon and the editor meet. Its WebSocket admits the origins the
// API's CORS admits, as the monitor's does.
func withEditor(api http.Handler, allowedOrigins []string) (http.Handler, error) {
	edit, err := editor.New(editor.Config{
		Logger:      slog.Default(),
		CheckOrigin: handlers.CheckOrigin(allowedOrigins),
	})
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle("GET /v1/edit", edit)
	mux.Handle("/", api)
	return mux, nil
}
