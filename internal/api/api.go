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
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
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
	servicesIndex *servicesindex.ServicesIndex,
	brickService *bricks.Service,
	idProvider *app.IDProvider,
	platform platform.Platform,
	cfg config.Configuration,
	allowedOrigins []string,
) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /debug/", http.DefaultServeMux) // pprof endpoints

	mux.Handle("GET /v1/version", handlers.HandlerVersion(version))
	mux.Handle("GET /v1/config", handlers.HandleConfig(cfg))
	mux.Handle("GET /v1/bricks", handlers.HandleBrickList(brickService))
	mux.Handle("GET /v1/bricks/{brickID}", handlers.HandleBrickDetails(brickService, idProvider, cfg))

	mux.Handle("GET /v1/properties", handlers.HandlePropertyKeys(cfg))
	mux.Handle("GET /v1/properties/{key}", handlers.HandlePropertyGet(cfg))
	mux.Handle("PUT /v1/properties/{key}", handlers.HandlePropertyUpsert(cfg))
	mux.Handle("DELETE /v1/properties/{key}", handlers.HandlePropertyDelete(cfg))

	mux.Handle("GET /v1/system/update/check", handlers.HandleCheckUpgradable(updater))
	mux.Handle("GET /v1/system/update/events", handlers.HandleUpdateEvents(updater))
	mux.Handle("PUT /v1/system/update/apply", handlers.HandleUpdateApply(updater))
	mux.Handle("GET /v1/system/resources", handlers.HandleSystemResources(cfg))

	mux.Handle("GET /v1/models", handlers.HandleModelsList(modelsIndex))
	mux.Handle("GET /v1/models/{modelID}", handlers.HandlerModelByID(modelsIndex))
	mux.Handle("PUT /v1/models/ei/projects/{projectID}", handlers.HandleInstallEIModel(cfg, bricksIndex, modelsIndex, dockerClient))
	mux.Handle("DELETE /v1/models/{modelID}", handlers.HandlerDeleteModelByID(dockerClient, cfg, modelsIndex, bricksIndex, idProvider, platform))

	mux.Handle("GET /v1/apps", handlers.HandleAppList(dockerClient, idProvider, bricksIndex, cfg))
	mux.Handle("POST /v1/apps", handlers.HandleAppCreate(idProvider, cfg))
	mux.Handle("GET /v1/apps/events", handlers.HandlerAppStatus(dockerClient, idProvider, bricksIndex, cfg))
	mux.Handle("GET /v1/apps/{appID}", handlers.HandleAppDetails(dockerClient, bricksIndex, idProvider, cfg))
	mux.Handle("PATCH /v1/apps/{appID}", handlers.HandleAppDetailsEdits(dockerClient, bricksIndex, idProvider, cfg))
	mux.Handle("GET /v1/apps/{appID}/logs", handlers.HandleAppLogs(dockerClient, idProvider, bricksIndex))
	mux.Handle("POST /v1/apps/{appID}/start", handlers.HandleAppStart(dockerClient, provisioner, modelsIndex, bricksIndex, servicesIndex, idProvider, cfg, staticStore, platform))
	mux.Handle("POST /v1/apps/{appID}/stop", handlers.HandleAppStop(dockerClient, idProvider, platform))
	mux.Handle("POST /v1/apps/{appID}/clone", handlers.HandleAppClone(dockerClient, idProvider, cfg))
	mux.Handle("DELETE /v1/apps/{appID}", handlers.HandleAppDelete(dockerClient, idProvider, platform))
	mux.Handle("GET /v1/apps/{appID}/export", handlers.HandleAppExport(cfg, idProvider, bricksIndex))
	mux.Handle("POST /v1/apps/import", handlers.HandleAppImport(cfg, idProvider))
	mux.Handle("GET /v1/apps/{appID}/exposed-ports", handlers.HandleAppPorts(bricksIndex, idProvider))
	mux.Handle("PUT /v1/apps/{appID}/sketch/libraries/{libRef}", handlers.HandleSketchAddLibrary(idProvider))
	mux.Handle("DELETE /v1/apps/{appID}/sketch/libraries/{libRef}", handlers.HandleSketchRemoveLibrary(idProvider))
	mux.Handle("GET /v1/apps/{appID}/sketch/libraries", handlers.HandleSketchListLibraries(idProvider))

	mux.Handle("GET /v1/apps/{appID}/bricks", handlers.HandleAppBrickInstancesList(brickService, idProvider))
	mux.Handle("GET /v1/apps/{appID}/bricks/{brickID}", handlers.HandleAppBrickInstanceDetails(brickService, idProvider))
	mux.Handle("PUT /v1/apps/{appID}/bricks/{brickID}", handlers.HandleBrickCreate(brickService, idProvider))
	mux.Handle("PATCH /v1/apps/{appID}/bricks/{brickID}", handlers.HandleBrickUpdates(brickService, idProvider))
	mux.Handle("DELETE /v1/apps/{appID}/bricks/{brickID}", handlers.HandleBrickDelete(brickService, idProvider))
	mux.Handle("POST /v1/apps/{appID}/bricks", handlers.HandleAppLocalBrickCreate(idProvider))
	mux.Handle("POST /v1/apps/{appID}/bricks/{brickID}/rename", handlers.HandleAppLocalBrickRename(brickService, idProvider))

	mux.Handle("GET /v1/docs/", http.StripPrefix("/v1/docs/", handlers.DocsServer(docsFS)))

	mux.Handle("GET /v1/monitor/ws", handlers.HandleMonitorWS(allowedOrigins))

	mux.Handle("GET /v1/libraries", handlers.HandleLibraryList(cfg.LibrariesAPIURL, version))

	return mux
}
