// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/arduino/go-paths-helper"
	"github.com/docker/cli/cli/command"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type ReleaseInstallResponse struct {
	ID      string `json:"id"`
	AppName string `json:"app_name,omitempty"`
}

// HandleReleaseInstall installs an uploaded Arduino App Release (.tar.gz) into the apps
// directory, ready to be launched with the app start endpoint. Unless 'prepare' is false,
// it also pre-pulls the app's container images (this blocks until the pull completes).
func HandleReleaseInstall(
	dockerClient command.Cli,
	cfg config.Configuration,
	idProvider *app.IDProvider,
	bricksIndex *bricksindex.BricksIndex,
	modelsIndex *modelsindex.ModelsIndex,
	plat platform.Platform,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		force := false
		if val := r.URL.Query().Get("force"); val != "" {
			parsed, err := strconv.ParseBool(val)
			if err != nil {
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "The parameter 'force' must be a boolean."})
				return
			}
			force = parsed
		}

		prepare := true
		if val := r.URL.Query().Get("prepare"); val != "" {
			parsed, err := strconv.ParseBool(val)
			if err != nil {
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "The parameter 'prepare' must be a boolean."})
				return
			}
			prepare = parsed
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			slog.Error("missing file parameter", "err", err)
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "missing required file parameter"})
			return
		}
		defer file.Close()

		tempFile, err := paths.MkTempFile(nil, "app-release-install-*.tar.gz")
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

		appID, manifest, err := orchestrator.InstallRelease(r.Context(), cfg, tempFilePath, idProvider, bricksIndex, modelsIndex, plat, force)
		if err != nil {
			slog.Error("release install failed", "file", header.Filename, "err", err)
			switch {
			case errors.Is(err, orchestrator.ErrIncompatibleRelease):
				render.EncodeResponse(w, http.StatusConflict, models.ErrorResponse{Details: err.Error()})
			case errors.Is(err, orchestrator.ErrAppAlreadyExists):
				render.EncodeResponse(w, http.StatusConflict, models.ErrorResponse{Details: err.Error()})
			case errors.Is(err, orchestrator.ErrBadRequest):
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: err.Error()})
			default:
				render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "failed to process the release: " + err.Error()})
			}
			return
		}

		// Install is committed on disk. Pre-pulling images is best-effort: a failure here must
		// not fail the install (images will be pulled on first start, or via the prepare endpoint).
		if prepare {
			if installedApp, loadErr := app.Load(appID.ToPath()); loadErr != nil {
				slog.Warn("release installed but could not load it to prepare images", "app_id", appID.String(), "err", loadErr)
			} else if prepErr := orchestrator.PrepareRelease(r.Context(), installedApp, dockerClient, func(orchestrator.StreamMessage) {}); prepErr != nil {
				slog.Warn("release installed but preparing container images failed", "app_id", appID.String(), "err", prepErr)
			}
		}

		resp := ReleaseInstallResponse{ID: appID.String()}
		if manifest != nil {
			resp.AppName = manifest.AppName
		}
		render.EncodeResponse(w, http.StatusCreated, resp)
	}
}
