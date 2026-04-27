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
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/docker/cli/cli/command"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type AppListResponse struct {
	Apps       []orchestrator.AppInfo       `json:"apps" description:"List of applications"`
	BrokenApps []orchestrator.BrokenAppInfo `json:"broken_apps,omitempty" description:"List of applications that are broken and couldn't be parsed"`
}

func HandleAppList(
	dockerCli command.Cli,
	idProvider *app.IDProvider,
	bricksIndex *bricksindex.BricksIndex,
	cfg config.Configuration,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		queryParams := r.URL.Query()

		showExamples, showApps, showOnlyDefault := true, true, false
		if filter := queryParams.Get("filter"); filter != "" {
			filters := strings.Split(strings.TrimSpace(filter), ",")
			showExamples = slices.Contains(filters, "examples")
			showOnlyDefault = slices.Contains(filters, "default")
			showApps = slices.Contains(filters, "apps")
		}

		var statusFilter orchestrator.Status
		if status := queryParams.Get("status"); status != "" {
			status, err := orchestrator.ParseStatus(status)
			if err != nil {
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "invalid status filter"})
				return
			}
			statusFilter = status
		}

		res, err := orchestrator.ListApps(r.Context(), dockerCli, orchestrator.ListAppRequest{
			ShowApps:        showApps,
			ShowExamples:    showExamples,
			ShowOnlyDefault: showOnlyDefault,
			StatusFilter:    statusFilter,
		}, idProvider, bricksIndex, cfg)
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})

			return
		}
		render.EncodeResponse(w, http.StatusOK, AppListResponse{Apps: res.Apps, BrokenApps: res.BrokenApps})
	}
}
