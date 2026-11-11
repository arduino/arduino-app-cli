// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"

	"github.com/docker/cli/cli/command"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/appbook"
	"github.com/arduino/arduino-app-cli/internal/appbook/httpapi"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type AppsStatusResponse struct {
	Apps []appbook.AppInfo `json:"apps" description:"The apps started at least once, with their status"`
}

// HandleAppList is the catalog with the status of each app: GET /v2/apps and GET /v2/apps/status in one.
func HandleAppList(
	dockerCli command.Cli,
	idProvider *appid.Provider,
	bricksIndex *bricksindex.BricksIndex,
	cfg config.Configuration,
	platform platform.Platform,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var statusFilter appbook.Status
		if status := r.URL.Query().Get("status"); status != "" {
			status, err := appbook.ParseStatus(status)
			if err != nil {
				render.EncodeResponse(w, http.StatusBadRequest, render.ErrorResponse{Details: "invalid status filter"})
				return
			}
			statusFilter = status
		}

		res, err := orchestrator.ListApps(r.Context(), dockerCli, httpapi.ListAppRequest(r), idProvider, bricksIndex, cfg, platform)
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, render.ErrorResponse{Details: "unable to find the app"})
			return
		}
		if statusFilter != "" {
			res.Apps = f.Filter(res.Apps, func(a appbook.AppInfo) bool { return a.Status == statusFilter })
		}
		render.EncodeResponse(w, http.StatusOK, httpapi.AppListResponse{Apps: res.Apps, BrokenApps: res.BrokenApps})
	}
}

// HandleAppsStatus lists the apps started at least once, with their status, as `app ps` does.
func HandleAppsStatus(dockerCli command.Cli, idProvider *appid.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apps, err := orchestrator.ListActiveApps(r.Context(), dockerCli, idProvider)
		if err != nil {
			slog.Error("Unable to list the apps status", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, render.ErrorResponse{Details: "unable to list the apps status"})
			return
		}
		render.EncodeResponse(w, http.StatusOK, AppsStatusResponse{Apps: apps})
	}
}
