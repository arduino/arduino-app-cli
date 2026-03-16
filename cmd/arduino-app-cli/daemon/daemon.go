// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package daemon

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jub0bs/cors"
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/app"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/internal/api"
	"github.com/arduino/arduino-app-cli/internal/httprecover"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/update"
	"github.com/arduino/arduino-app-cli/internal/update/apt"
	"github.com/arduino/arduino-app-cli/internal/update/arduino"
)

func NewDaemonCmd(cfg config.Configuration, version string) *cobra.Command {
	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run the Arduino App CLI as an HTTP daemon",
		Run: func(cmd *cobra.Command, args []string) {
			daemonPort, _ := cmd.Flags().GetString("port")

			if err := stopArduinoFailingApps(cmd.Context(), cfg); err != nil {
				slog.Warn("Failed to stop containers", "error", err.Error())
			}

			// start the default app in the background
			go func() {
				slog.Info("Starting default app")
				err := orchestrator.StartDefaultApp(
					cmd.Context(),
					servicelocator.GetDockerClient(),
					servicelocator.GetProvisioner(),
					servicelocator.GetModelsIndex(),
					servicelocator.GetBricksIndex(),
					servicelocator.GetAppIDProvider(),
					cfg,
					servicelocator.GetStaticStore(),
					servicelocator.GetPlatform(),
				)
				if err != nil {
					slog.Error("Failed to start default app", slog.String("error", err.Error()))
				} else {
					slog.Info("Default app started")
				}
			}()

			httpHandler(cmd.Context(), cfg, daemonPort, version)
		},
	}
	daemonCmd.Flags().String("port", "8080", "The TCP port the daemon will listen to")
	return daemonCmd
}

func httpHandler(ctx context.Context, cfg config.Configuration, daemonPort, version string) {
	slog.Info("Starting HTTP server", slog.String("address", ":"+daemonPort))

	corsConfig := cors.Config{
		Origins: []string{
			"wails://wails",
			"wails://wails.localhost:*",
			"http://wails.localhost:*",
			"http://localhost:*",
			"https://localhost:*",
		},
		Methods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodOptions,
			http.MethodDelete,
			http.MethodPatch,
		},
		RequestHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-API-Key",
		},
		MaxAgeInSeconds: 86400,
		ResponseHeaders: []string{},
	}

	apiSrv := api.NewHTTPRouter(
		servicelocator.GetDockerClient(),
		version,
		update.NewManager(
			apt.New(),
			arduino.NewArduinoPlatformUpdater(servicelocator.GetPlatform(), cfg.ArduinoPlatformVersionConstraint),
		),
		servicelocator.GetProvisioner(),
		servicelocator.GetStaticStore(),
		servicelocator.GetModelsIndex(),
		servicelocator.GetBricksIndex(),
		servicelocator.GetBrickService(),
		servicelocator.GetAppIDProvider(),
		servicelocator.GetPlatform(),
		cfg,
		corsConfig.Origins,
	)

	// Wrap the API server with CORS middleware
	corsMiddlware, err := cors.NewMiddleware(corsConfig)
	if err != nil {
		panic(err)
	}
	apiSrv = corsMiddlware.Wrap(apiSrv)

	// Start the HTTP server
	address := "127.0.0.1:" + daemonPort
	httpSrv := http.Server{
		Addr:              address,
		Handler:           httprecover.RecoverPanic(apiSrv),
		ReadHeaderTimeout: 60 * time.Second,
	}
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(err.Error())
		}
	}()

	<-ctx.Done()
	slog.Info("Shutting down HTTP server", slog.String("address", address))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	_ = httpSrv.Shutdown(ctx)
	cancel()
	slog.Info("HTTP server shut down", slog.String("address", address))
}

// stopArduinoFailingApps force stop of arduino apps with container in auto-restart.
func stopArduinoFailingApps(ctx context.Context, config config.Configuration) error {
	docker := servicelocator.GetDockerClient()

	listOpts := orchestrator.ListAppRequest{
		ShowApps: true, ShowExamples: true,
		StatusFilter: orchestrator.StatusFailed,
	}
	apps, err := orchestrator.ListApps(ctx, docker, listOpts, servicelocator.GetAppIDProvider(), config)
	if err != nil {
		return err
	}
	slog.Info("Found apps with failed status", "count", len(apps.Apps))

	for _, appInfo := range apps.Apps {
		app, err := app.Load(appInfo.ID.ToPath().String())
		if err != nil {
			slog.Warn("Failed to load app", "appID", appInfo.ID.String(), "error", err.Error())
			continue
		}

		slog.Info("Stopping app container", "appID", appInfo.ID.String())
		for event := range orchestrator.StopApp(ctx, docker, servicelocator.GetPlatform(), app) {
			if event.IsError() {
				slog.Warn("Failed to stop app", "appID", appInfo.ID.String(), "error", event.GetError().Error())
			}
		}
	}

	return nil
}
