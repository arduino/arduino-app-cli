// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package httpapi

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/appbook"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type AppImportResponse struct {
	ID string `json:"id"`
}

func HandleAppImport(
	cfg config.Configuration,
	idProvider *appid.Provider,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, header, err := r.FormFile("file")
		if err != nil {
			slog.Error("missing file parameter", "err", err)
			render.EncodeResponse(w, http.StatusBadRequest, render.ErrorResponse{Details: "missing required file parameter"})
			return
		}
		defer file.Close()

		tempFile, err := paths.MkTempFile(nil, "app-import-*.zip")
		if err != nil {
			slog.Error("unable to create temp file", "err", err)
			render.EncodeResponse(w, http.StatusInternalServerError, render.ErrorResponse{Details: "internal server error"})
			return
		}

		tempFilePath := paths.NewFromFile(tempFile)
		defer func() { _ = tempFilePath.Remove() }()

		if _, err := io.Copy(tempFile, file); err != nil {
			tempFile.Close()
			slog.Error("unable to save upload to temp file", "err", err)
			render.EncodeResponse(w, http.StatusInternalServerError, render.ErrorResponse{Details: "failed to save uploaded file"})
			return
		}
		tempFile.Close()

		appID, err := appbook.ImportAppFromZip(cfg, tempFilePath, idProvider, header.Filename)
		if err != nil {
			slog.Error("import failed", "err", err)

			switch {
			case errors.Is(err, appbook.ErrAppAlreadyExists):
				render.EncodeResponse(w, http.StatusConflict, render.ErrorResponse{Details: err.Error()})
			case errors.Is(err, appbook.ErrBadRequest) || strings.Contains(err.Error(), "not a valid zip file"):
				render.EncodeResponse(w, http.StatusBadRequest, render.ErrorResponse{Details: err.Error()})
			default:
				render.EncodeResponse(w, http.StatusInternalServerError, render.ErrorResponse{Details: "failed to process the archive: " + err.Error()})
			}
			return
		}

		render.EncodeResponse(w, http.StatusCreated, AppImportResponse{ID: appID.String()})
	}
}
