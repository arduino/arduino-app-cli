// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
// ... (License header standard) ...

package handlers

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type ImportResponse struct {
	ID string `json:"id"`
}

func HandleAppImport(
	cfg config.Configuration,
	idProvider *app.IDProvider,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		folderName := strings.TrimSpace(r.FormValue("folder_name"))
		if folderName == "" {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "missing required 'folder_name' parameter"})
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			slog.Error("missing file parameter", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "missing required file parameter"})
			return
		}
		defer file.Close()

		tempFile, err := os.CreateTemp("", "app-import-*.zip")
		if err != nil {
			slog.Error("unable to create temp file", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "internal server error"})
			return
		}
		tempPath := tempFile.Name()
		defer os.Remove(tempPath)

		if _, err := io.Copy(tempFile, file); err != nil {
			tempFile.Close()
			slog.Error("unable to save upload to temp file", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "failed to save uploaded file"})
			return
		}
		tempFile.Close()

		appID, err := orchestrator.ImportAppFromZip(cfg, tempPath, folderName, idProvider)
		if err != nil {
			handleImportError(w, err)
			return
		}

		slog.Info("app imported successfully", slog.String("app_id", appID))
		render.EncodeResponse(w, http.StatusCreated, ImportResponse{ID: appID})
	}
}

func handleImportError(w http.ResponseWriter, err error) {
	slog.Error("import failed", slog.String("error", err.Error()))

	if errors.Is(err, orchestrator.ErrAppAlreadyExists) {
		render.EncodeResponse(w, http.StatusConflict, models.ErrorResponse{Details: err.Error()})
		return
	}
	if errors.Is(err, orchestrator.ErrBadRequest) ||
		strings.Contains(err.Error(), "not a valid zip file") {
		render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: err.Error()})
		return
	}

	render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "failed to process the archive: " + err.Error()})
}
