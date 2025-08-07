package handlers

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/docker/cli/cli/command"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/pkg/render"
)

type AppListResponse struct {
	Apps []orchestrator.AppInfo `json:"apps" description:"List of applications"`
}

func HandleAppList(dockerCli command.Cli) http.HandlerFunc {
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
		})
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})

			return
		}
		render.EncodeResponse(w, http.StatusOK, AppListResponse{Apps: res.Apps})
	}
}
