package api

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/api/handlers"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/update"

	dockerClient "github.com/docker/docker/client"
)

//go:embed docs
var docsFS embed.FS

func NewHTTPRouter(
	dockerClient *dockerClient.Client,
	version string,
	updater *update.Manager,
	provisioner *orchestrator.Provision,
	bricksDocsFS fs.FS,
	modelsIndex *modelsindex.ModelsIndex,
	bricksIndex *bricksindex.BricksIndex,
) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /v1/version", handlers.HandlerVersion(version))
	mux.Handle("GET /v1/config", handlers.HandleConfig())
	mux.Handle("GET /v1/bricks", handlers.HandleBrickList(modelsIndex, bricksIndex))
	mux.Handle("GET /v1/bricks/{brickID}", handlers.HandleBrickDetails(bricksDocsFS, bricksIndex))

	mux.Handle("GET /v1/system/update/check", handlers.HandleCheckUpgradable(updater))
	mux.Handle("GET /v1/system/update/events", handlers.HandleUpdateEvents(updater))
	mux.Handle("PUT /v1/system/update/apply", handlers.HandleUpdateApply(updater))
	mux.Handle("GET /v1/system/resources", handlers.HandleSystemResources())

	mux.Handle("GET /v1/models", handlers.HandleModelsList(modelsIndex))
	mux.Handle("GET /v1/models/{modelID}", handlers.HandlerModelByID(modelsIndex))

	mux.Handle("GET /v1/apps", handlers.HandleAppList(dockerClient))
	mux.Handle("POST /v1/apps", handlers.HandleAppCreate(dockerClient))

	mux.Handle("GET /v1/apps/{appID}", handlers.HandleAppDetails(dockerClient, bricksIndex))
	mux.Handle("PATCH /v1/apps/{appID}", handlers.HandleAppDetailsEdits())
	mux.Handle("GET /v1/apps/{appID}/logs", handlers.HandleAppLogs(dockerClient))
	mux.Handle("POST /v1/apps/{appID}/start", handlers.HandleAppStart(dockerClient, provisioner, modelsIndex, bricksIndex))
	mux.Handle("POST /v1/apps/{appID}/stop", handlers.HandleAppStop(dockerClient))
	mux.Handle("POST /v1/apps/{appID}/clone", handlers.HandleAppClone(dockerClient))
	mux.Handle("DELETE /v1/apps/{appID}", handlers.HandleAppDelete())

	mux.Handle("GET /v1/apps/{appID}/bricks", handlers.HandleAppBrickInstancesList(bricksIndex))
	mux.Handle("GET /v1/apps/{appID}/bricks/{brickID}", handlers.HandleAppBrickInstanceDetails(bricksIndex))
	mux.Handle("PUT /v1/apps/{appID}/bricks/{brickID}", handlers.HandleBrickCreate(modelsIndex, bricksIndex))
	mux.Handle("PATCH /v1/apps/{appID}/bricks/{brickID}", handlers.HandleBrickUpdates(modelsIndex, bricksIndex))
	mux.Handle("DELETE /v1/apps/{appID}/bricks/{brickID}", handlers.HandleBrickDelete(bricksIndex))

	mux.Handle("GET /v1/docs/", http.StripPrefix("/v1/docs/", handlers.DocsServer(docsFS)))

	return mux
}
