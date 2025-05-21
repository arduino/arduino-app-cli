package handlers

import (
	"log/slog"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/pkg/render"

	dockerClient "github.com/docker/docker/client"
)

func HandleAppList(dockerClient *dockerClient.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		queryParams := r.URL.Query()

		showExamples := queryParams.Get("example") == "true"
		showOnlyDefault := queryParams.Get("default") == "true"

		var statusFilter string
		if status := queryParams.Get("status"); status != "" {
			statusFilter = status
		}

		res, err := orchestrator.ListApps(r.Context(), orchestrator.ListAppRequest{
			ShowExamples:    showExamples,
			ShowOnlyDefault: showOnlyDefault,
			StatusFilter:    statusFilter,
		})
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, "unable to find the app")
			return
		}
		render.EncodeResponse(w, http.StatusOK, res)
	}
}
