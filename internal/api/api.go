package api

import (
	"embed"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/api/handlers"

	dockerClient "github.com/docker/docker/client"
)

//go:embed docs
var docsFS embed.FS

func NewHTTPRouter(dockerClient *dockerClient.Client, version string, eventsBroker *handlers.UpdateEventsBroker) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /v1/version", handlers.HandlerVersion(version))
	mux.Handle("GET /v1/config", handlers.HandleConfig())
	mux.Handle("GET /v1/bricks", handlers.HandleBrickList())
	mux.Handle("GET /v1/bricks/{brickID}", handlers.HandleBrickDetails())
	mux.Handle("GET /v1/system/update/check", handlers.HandleCheckUpgradable())
	mux.Handle("GET /v1/system/update/events", handlers.HandleUpdateEvents(eventsBroker))
	mux.Handle("PUT /v1/system/update/apply", handlers.HandleUpdateApply(eventsBroker))

	mux.Handle("GET /v1/models", handlers.HandleModelsList())
	mux.Handle("GET /v1/models/{modelID}", handlers.HandlerModelByID())

	mux.Handle("GET /v1/apps", handlers.HandleAppList(dockerClient))
	mux.Handle("POST /v1/apps", handlers.HandleAppCreate(dockerClient))

	mux.Handle("GET /v1/apps/{appID}", handlers.HandleAppDetails(dockerClient))
	mux.Handle("PATCH /v1/apps/{appID}", handlers.HandleAppDetailsEdits())
	mux.Handle("GET /v1/apps/{appID}/logs", handlers.HandleAppLogs(dockerClient))
	mux.Handle("POST /v1/apps/{appID}/start", handlers.HandleAppStart(dockerClient))
	mux.Handle("POST /v1/apps/{appID}/stop", handlers.HandleAppStop(dockerClient))
	mux.Handle("POST /v1/apps/{appID}/clone", handlers.HandleAppClone(dockerClient))
	mux.Handle("DELETE /v1/apps/{appID}", handlers.HandleAppDelete())

	mux.Handle("GET /v1/docs/", http.StripPrefix("/v1/docs/", handlers.DocsServer(docsFS)))

	return mux
}
