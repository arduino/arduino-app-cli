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
	"strings"

	"github.com/docker/cli/cli/command"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/render"
)

// brickUninstalled is the response of a brick uninstall.
type brickUninstalled struct {
	ID       string   `json:"id"`
	Versions []string `json:"versions"`
	// Warnings are the images of the brick that could not be removed, and why.
	Warnings []string `json:"warnings"`
}

// HandleUninstallBrick uninstalls a third-party brick: the version of the version query parameter
// or, without it, every installed version. A brick used by a running app is not uninstalled.
func HandleUninstallBrick(
	dockerClient command.Cli,
	cfg config.Configuration,
	bricksIndex *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
	modelsIndex *modelsindex.ModelsIndex,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("brickID")
		if err := bricksindex.ValidateBrickID(id); err != nil {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: err.Error()})
			return
		}
		if !bricksindex.IsThirdParty(id) {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: fmt.Sprintf("the brick %s is built-in and cannot be uninstalled", id)})
			return
		}
		var version *semver.Version
		if query := r.URL.Query(); query.Has("version") {
			v, err := bricksindex.ParseBrickVersion(query.Get("version"))
			if err != nil {
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: fmt.Sprintf("invalid version: %v", err)})
				return
			}
			version = v
		}

		warnings := []string{}
		res, err := orchestrator.UninstallBrick(r.Context(), dockerClient, cfg, bricksIndex, servicesIndex, modelsIndex, id, version,
			func(msg string) { warnings = append(warnings, msg) })
		if errors.Is(err, orchestrator.ErrBrickInUse) {
			render.EncodeResponse(w, http.StatusConflict, models.ErrorResponse{Details: err.Error()})
			return
		}
		if err != nil {
			slog.Error("unable to uninstall brick", "brick", id, "error", err)
			details := err.Error()
			// The versions removed before the failure are gone, and the client has to know.
			if len(res.Versions) > 0 {
				details = fmt.Sprintf("%s (uninstalled: %s)", details, strings.Join(res.Versions, ", "))
			}
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: details})
			return
		}
		if len(res.Versions) == 0 {
			ref := id
			if version != nil {
				ref += "@" + version.String()
			}
			render.EncodeResponse(w, http.StatusNotFound, models.ErrorResponse{Details: fmt.Sprintf("the brick %s is not installed", ref)})
			return
		}
		render.EncodeResponse(w, http.StatusOK, brickUninstalled{ID: id, Versions: res.Versions, Warnings: warnings})
	}
}
