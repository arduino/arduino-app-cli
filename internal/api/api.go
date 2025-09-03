package api

import (
	"embed"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/api/handlers"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricks"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/store"
	"github.com/arduino/arduino-app-cli/internal/update"

	"github.com/docker/cli/cli/command"

	_ "net/http/pprof" //nolint:gosec // pprof import is safe for profiling endpoints
)

//go:embed docs
var docsFS embed.FS

func NewHTTPRouter(
	dockerClient command.Cli,
	version string,
	updater *update.Manager,
	provisioner *orchestrator.Provision,
	staticStore *store.StaticStore,
	modelsIndex *modelsindex.ModelsIndex,
	bricksIndex *bricksindex.BricksIndex,
	brickService *bricks.Service,
	idProvider *app.IDProvider,
	cfg config.Configuration,
	allowedOrigins []string,
) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /debug/", http.DefaultServeMux) // pprof endpoints

	mux.Handle("GET /v1/version", handlers.HandlerVersion(version))
	mux.Handle("GET /v1/config", handlers.HandleConfig(cfg))
	mux.Handle("GET /v1/bricks", handlers.HandleBrickList(brickService))
	mux.Handle("GET /v1/bricks/{brickID}", handlers.HandleBrickDetails(brickService))

	mux.Handle("GET /v1/system/update/check", handlers.HandleCheckUpgradable(updater))
	mux.Handle("GET /v1/system/update/events", handlers.HandleUpdateEvents(updater))
	mux.Handle("PUT /v1/system/update/apply", handlers.HandleUpdateApply(updater))
	mux.Handle("GET /v1/system/resources", handlers.HandleSystemResources())

	mux.Handle("GET /v1/models", handlers.HandleModelsList(modelsIndex))
	mux.Handle("GET /v1/models/{modelID}", handlers.HandlerModelByID(modelsIndex))

	mux.Handle("GET /v1/apps", handlers.HandleAppList(dockerClient, idProvider, cfg))
	mux.Handle("POST /v1/apps", handlers.HandleAppCreate(idProvider, cfg))

	mux.Handle("GET /v1/apps/{appID}", handlers.HandleAppDetails(dockerClient, bricksIndex, idProvider, cfg))
	mux.Handle("PATCH /v1/apps/{appID}", handlers.HandleAppDetailsEdits(dockerClient, bricksIndex, idProvider, cfg))
	mux.Handle("GET /v1/apps/{appID}/logs", handlers.HandleAppLogs(dockerClient, idProvider, staticStore))
	mux.Handle("POST /v1/apps/{appID}/start", handlers.HandleAppStart(dockerClient, provisioner, modelsIndex, bricksIndex, idProvider, cfg, staticStore))
	mux.Handle("POST /v1/apps/{appID}/stop", handlers.HandleAppStop(dockerClient, idProvider))
	mux.Handle("POST /v1/apps/{appID}/clone", handlers.HandleAppClone(dockerClient, idProvider, cfg))
	mux.Handle("DELETE /v1/apps/{appID}", handlers.HandleAppDelete(idProvider))
	mux.Handle("GET /v1/apps/{appID}/exposed-ports", handlers.HandleAppPorts(bricksIndex, idProvider))

	mux.Handle("GET /v1/apps/{appID}/bricks", handlers.HandleAppBrickInstancesList(brickService, idProvider))
	mux.Handle("GET /v1/apps/{appID}/bricks/{brickID}", handlers.HandleAppBrickInstanceDetails(brickService, idProvider))
	mux.Handle("PUT /v1/apps/{appID}/bricks/{brickID}", handlers.HandleBrickCreate(brickService, idProvider))
	mux.Handle("PATCH /v1/apps/{appID}/bricks/{brickID}", handlers.HandleBrickUpdates(brickService, idProvider))
	mux.Handle("DELETE /v1/apps/{appID}/bricks/{brickID}", handlers.HandleBrickDelete(brickService, idProvider))

	mux.Handle("GET /v1/docs/", http.StripPrefix("/v1/docs/", handlers.DocsServer(docsFS)))

	mux.Handle("GET /v1/monitor/ws", handlers.HandleMonitorWS(allowedOrigins))

	return mux
}
