// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

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
