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
	"strings"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-app-cli/internal/render"
)

// HandleAppReleaseBuild builds a reproducible Arduino App Release from a pre-built app and
// streams it back as a .tar.gz download.
func HandleAppReleaseBuild(
	bricksIndex *bricksindex.BricksIndex,
	modelsIndex *modelsindex.ModelsIndex,
	idProvider *app.IDProvider,
	cfg config.Configuration,
	plat platform.Platform,
	version string,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: fmt.Sprintf("invalid id: %s", err.Error())})
			return
		}

		appToRelease, err := app.Load(id.ToPath())
		if err != nil {
			slog.Error("Unable to load the app", "error", err.Error(), "path", id.String())
			if errors.Is(err, os.ErrNotExist) {
				render.EncodeResponse(w, http.StatusNotFound, models.ErrorResponse{Details: err.Error()})
			} else {
				render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: err.Error()})
			}
			return
		}

		tempFile, err := paths.MkTempFile(nil, "app-release-*.tar.gz")
		if err != nil {
			slog.Error("unable to create temp file", "err", err)
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "internal server error"})
			return
		}
		tempFile.Close()
		tempFilePath := paths.New(tempFile.Name())
		defer func() { _ = tempFilePath.Remove() }()

		includeModels := true
		if val := r.URL.Query().Get("models"); val != "" {
			parsed, err := strconv.ParseBool(val)
			if err != nil {
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "The parameter 'models' must be a boolean."})
				return
			}
			includeModels = parsed
		}

		releaseNumber := r.URL.Query().Get("release_number")

		keepSecrets := false
		if val := r.URL.Query().Get("keep_secrets"); val != "" {
			parsed, err := strconv.ParseBool(val)
			if err != nil {
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "The parameter 'keep_secrets' must be a boolean."})
				return
			}
			keepSecrets = parsed
		}

		if err := orchestrator.BuildRelease(r.Context(), bricksIndex, modelsIndex, appToRelease, cfg, plat, version, releaseNumber, tempFilePath, includeModels, keepSecrets, nil); err != nil {
			slog.Error("failed to build release", "app_id", id.String(), "error", err)
			if errors.Is(err, orchestrator.ErrBadRequest) {
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: err.Error()})
			} else {
				render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "failed to build release"})
			}
			return
		}

		f, err := os.Open(tempFilePath.String())
		if err != nil {
			slog.Error("failed to open generated release", "error", err)
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "failed to read generated release"})
			return
		}
		defer f.Close()

		// Stream the archive back rather than buffering the whole file (releases can bundle
		// hundreds of MB of AI models, which would risk OOM on the device serving the API).
		render.EncodeTarGzStream(w, http.StatusOK, f, releaseDownloadName(appToRelease.Name))
	}
}

func releaseDownloadName(appName string) string {
	name := strings.ToLower(strings.ReplaceAll(appName, " ", "-"))
	if name == "" {
		name = "app-release"
	}
	return name + ".tar.gz"
}
