package api

import (
	"net/http"
	"strings"

	"github.com/arduino/arduino-app-cli/internal/api/handlers"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/pkg/render"

	dockerClient "github.com/docker/docker/client"
)

func NewHTTPRouter(dockerClient *dockerClient.Client, version string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/version", func(w http.ResponseWriter, r *http.Request) {
		render.EncodeResponse(w, http.StatusOK, struct {
			Version string `json:"version"`
		}{
			Version: version,
		})
	})

	mux.Handle("GET /v1/apps", handlers.HandleAppList(dockerClient))
	mux.Handle("POST /v1/apps", handlers.HandleAppCreate(dockerClient))

	appLogsHandler := handlers.HandleAppLogs(dockerClient)
	appEventsHandler := handlers.HandleAppEvents(dockerClient)
	appGetVariablesHandler := handlers.HandleAppGetVariables(dockerClient)
	appDetailsHandler := handlers.HandleAppDetails(dockerClient)
	mux.HandleFunc("GET /v1/apps/{path...}", func(w http.ResponseWriter, r *http.Request) {
		path := r.PathValue("path")
		switch {
		case strings.HasSuffix(path, "/logs"):
			id := strings.TrimSuffix(path, "/logs")
			appLogsHandler(w, r, orchestrator.ID(id))
		case strings.HasSuffix(path, "/events"):
			id := strings.TrimSuffix(path, "/events")
			appEventsHandler(w, r, orchestrator.ID(id))
		case strings.HasSuffix(path, "/variables"):
			id := strings.TrimSuffix(path, "/variables")
			appGetVariablesHandler(w, r, orchestrator.ID(id))
		default:
			appDetailsHandler(w, r, orchestrator.ID(path))
		}
	})

	appSetVariablesHandler := handlers.HandleAppSetVariables(dockerClient)
	mux.HandleFunc("PATCH /v1/apps/{path...}", func(w http.ResponseWriter, r *http.Request) {
		path := r.PathValue("path")
		switch {
		case strings.HasSuffix(path, "/variables"):
			id := strings.TrimSuffix(path, "/variables")
			appSetVariablesHandler(w, r, orchestrator.ID(id))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	startHandler := handlers.HandleAppStart(dockerClient)
	stopHandler := handlers.HandleAppStop(dockerClient)
	cloneHandler := handlers.HandleAppClone(dockerClient)
	mux.HandleFunc("POST /v1/apps/{path...}", func(w http.ResponseWriter, r *http.Request) {
		path := r.PathValue("path")
		switch {
		case strings.HasSuffix(path, "/start"):
			id := strings.TrimSuffix(path, "/start")
			startHandler(w, r, orchestrator.ID(id))
		case strings.HasSuffix(path, "/stop"):
			id := strings.TrimSuffix(path, "/stop")
			stopHandler(w, r, orchestrator.ID(id))
		case strings.HasSuffix(path, "/clone"):
			id := strings.TrimSuffix(path, "/clone")
			cloneHandler(w, r, orchestrator.ID(id))
		case strings.HasSuffix(path, "/default"):
			id := strings.TrimSuffix(path, "/default")
			handlers.HandleAppSetDefault(w, r, orchestrator.ID(id))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	deletehandler := handlers.HandleAppDelete()
	mux.HandleFunc("DELETE /v1/apps/{path...}", func(w http.ResponseWriter, r *http.Request) {
		path := r.PathValue("path")
		switch {
		case strings.HasSuffix(path, "/default"):
			id := strings.TrimSuffix(path, "/default")
			handlers.HandleDeleteDefault(w, r, orchestrator.ID(id))
		default:
			deletehandler(w, r, orchestrator.ID(path))
		}
	})

	return mux
}
