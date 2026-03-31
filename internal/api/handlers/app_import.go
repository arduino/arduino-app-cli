// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package handlers

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type AppImportResponse struct {
	ID string `json:"id"`
}

func HandleAppImport(
	cfg config.Configuration,
	idProvider *app.IDProvider,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, header, err := r.FormFile("file")
		if err != nil {
			slog.Error("missing file parameter", "err", err)
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "missing required file parameter"})
			return
		}
		defer file.Close()

		tempFile, err := paths.MkTempFile(nil, "app-import-*.zip")
		if err != nil {
			slog.Error("unable to create temp file", "err", err)
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "internal server error"})
			return
		}

		tempFilePath := paths.NewFromFile(tempFile)
		defer func() { _ = tempFilePath.Remove() }()

		if _, err := io.Copy(tempFile, file); err != nil {
			tempFile.Close()
			slog.Error("unable to save upload to temp file", "err", err)
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "failed to save uploaded file"})
			return
		}
		tempFile.Close()

		appID, err := orchestrator.ImportAppFromZip(cfg, tempFilePath, idProvider, header.Filename)
		if err != nil {
			slog.Error("import failed", "err", err)

			switch {
			case errors.Is(err, orchestrator.ErrAppAlreadyExists):
				render.EncodeResponse(w, http.StatusConflict, models.ErrorResponse{Details: err.Error()})
			case errors.Is(err, orchestrator.ErrBadRequest) || strings.Contains(err.Error(), "not a valid zip file"):
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: err.Error()})
			default:
				render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "failed to process the archive: " + err.Error()})
			}
			return
		}

		render.EncodeResponse(w, http.StatusCreated, AppImportResponse{ID: appID.String()})
	}
}
