// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/render"
)

func HandleAppExport(
	idProvider *appid.Provider,
	bricksIndex *bricksindex.BricksIndex,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: fmt.Sprintf("invalid id: %s", err.Error())})
			return
		}
		appToExport, err := app.Load(id.ToPath())
		if err != nil {
			slog.Error("Unable to load the app", "error", err.Error(), "path", id.String())
			if errors.Is(err, os.ErrNotExist) {
				render.EncodeResponse(w, http.StatusNotFound, models.ErrorResponse{Details: err.Error()})
			} else {
				render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: err.Error()})
			}
			return
		}

		includeData := false
		if val := r.URL.Query().Get("include_data"); val != "" {
			var err error
			includeData, err = strconv.ParseBool(val)
			if err != nil {
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{
					Details: "The parameter 'include_data' must be a boolean.",
				})
				return
			}
		}

		zipBytes, fileName, err := orchestrator.ExportAppZip(bricksIndex, appToExport, includeData)
		if err != nil {
			slog.Error("failed to export app", "app_id", id.String(), "error", err)
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{
				Details: "Failed to generate zip archive",
			})
			return
		}

		render.EncodeZipResponse(w, http.StatusOK, zipBytes, fileName)
	}
}
