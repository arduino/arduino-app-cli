// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
// ... (License header standard) ...

package handlers

import (
	"encoding/json"
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

type ImportOptions struct {
	FolderName string `json:"folder_name"`
}

type ImportResponse struct {
	ID string `json:"id"`
}

func HandleAppImport(
	cfg config.Configuration,
	idProvider *app.IDProvider,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, _, err := r.FormFile("file")
		if err != nil {
			slog.Error("missing file parameter", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "missing required file parameter"})
			return
		}
		defer file.Close()

		optionsStr := r.FormValue("options")
		if optionsStr == "" {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "missing required 'options' JSON parameter"})
			return
		}

		var opts ImportOptions
		if err := json.Unmarshal([]byte(optionsStr), &opts); err != nil {
			slog.Error("invalid options json", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "invalid 'options' JSON format"})
			return
		}

		if strings.TrimSpace(opts.FolderName) == "" {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "json field 'folder_name' is required"})
			return
		}

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

		appID, err := orchestrator.ImportAppFromZip(cfg, tempPath, opts.FolderName, idProvider)
		if err != nil {
			slog.Error("import failed", slog.String("error", err.Error()))

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

		slog.Info("app imported successfully", slog.String("app_id", appID))
		render.EncodeResponse(w, http.StatusCreated, ImportResponse{ID: appID})
	}
}
