package api

import (
	"embed"
	"net/http"
	"strings"

	"github.com/arduino/arduino-app-cli/internal/api/handlers"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/pkg/render"

	dockerClient "github.com/docker/docker/client"
)

//go:embed docs
var docsFS embed.FS

func NewHTTPRouter(dockerClient *dockerClient.Client, version string) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /v1/version", handlers.HandlerVersion(version))
	mux.Handle("GET /v1/config", handlers.HandleConfig())
	mux.Handle("GET /v1/bricks", handlers.HandleBrickList())
	mux.Handle("GET /v1/bricks/{id...}", handlers.HandleBrickDetails())
	mux.Handle("GET /v1/system/update/check", handlers.HandleCheckUpgradable())
	mux.Handle("GET /v1/system/update/apply", handlers.HandleUpgrade())

	mux.Handle("GET /v1/apps", handlers.HandleAppList(dockerClient))
	mux.Handle("POST /v1/apps", handlers.HandleAppCreate(dockerClient))

	appLogsHandler := handlers.HandleAppLogs(dockerClient)
	appEventsHandler := handlers.HandleAppEvents(dockerClient)
	appDetailsHandler := handlers.HandleAppDetails(dockerClient)
	mux.HandleFunc("GET /v1/apps/{path...}", func(w http.ResponseWriter, r *http.Request) {
		path := r.PathValue("path")
		switch {
		case strings.HasSuffix(path, "/logs"):
			id, err := orchestrator.ParseID(strings.TrimSuffix(path, "/logs"))
			if err != nil {
				render.EncodeResponse(w, http.StatusBadRequest, "invalid app ID")
				return
			}
			appLogsHandler(w, r, id)
		case strings.HasSuffix(path, "/events"):
			id, err := orchestrator.ParseID(strings.TrimSuffix(path, "/events"))
			if err != nil {
				render.EncodeResponse(w, http.StatusBadRequest, "invalid app ID")
				return
			}
			appEventsHandler(w, r, id)
		default:
			id, err := orchestrator.ParseID(path)
			if err != nil {
				render.EncodeResponse(w, http.StatusBadRequest, "invalid app ID")
				return
			}
			appDetailsHandler(w, r, id)
		}
	})

	appDetailsEditsHandler := handlers.HandleAppDetailsEdits()
	mux.HandleFunc("PATCH /v1/apps/{path...}", func(w http.ResponseWriter, r *http.Request) {
		id, err := orchestrator.ParseID(r.PathValue("path"))
		if err != nil {
			render.EncodeResponse(w, http.StatusBadRequest, "invalid app ID")
			return
		}
		appDetailsEditsHandler(w, r, id)
	})

	startHandler := handlers.HandleAppStart(dockerClient)
	stopHandler := handlers.HandleAppStop(dockerClient)
	cloneHandler := handlers.HandleAppClone(dockerClient)
	mux.HandleFunc("POST /v1/apps/{path...}", func(w http.ResponseWriter, r *http.Request) {
		path := r.PathValue("path")
		switch {
		case strings.HasSuffix(path, "/start"):
			id, err := orchestrator.ParseID(strings.TrimSuffix(path, "/start"))
			if err != nil {
				render.EncodeResponse(w, http.StatusBadRequest, "invalid app ID")
				return
			}
			startHandler(w, r, id)
		case strings.HasSuffix(path, "/stop"):
			id, err := orchestrator.ParseID(strings.TrimSuffix(path, "/stop"))
			if err != nil {
				render.EncodeResponse(w, http.StatusBadRequest, "invalid app ID")
				return
			}
			stopHandler(w, r, id)
		case strings.HasSuffix(path, "/clone"):
			id, err := orchestrator.ParseID(strings.TrimSuffix(path, "/clone"))
			if err != nil {
				render.EncodeResponse(w, http.StatusBadRequest, "invalid app ID")
				return
			}
			cloneHandler(w, r, id)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	deletehandler := handlers.HandleAppDelete()
	mux.HandleFunc("DELETE /v1/apps/{path...}", func(w http.ResponseWriter, r *http.Request) {
		id, err := orchestrator.ParseID(r.PathValue("path"))
		if err != nil {
			render.EncodeResponse(w, http.StatusBadRequest, "invalid app ID")
			return
		}
		deletehandler(w, r, id)
	})

	docsHandler := handlers.DocsServer(docsFS)
	mux.Handle("GET /v1/docs/", http.StripPrefix("/v1/docs/", docsHandler))

	return mux
}
