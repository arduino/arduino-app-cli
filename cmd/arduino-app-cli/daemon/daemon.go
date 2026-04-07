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

package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/docker/cli/cli/command"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/jub0bs/cors"
	"github.com/spf13/cobra"

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

			err := stopArduinoContainers(cmd.Context(), servicelocator.GetDockerClient())
			if err != nil {
				slog.Warn("Failed to stop containers", slog.String("error", err.Error()))
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
					servicelocator.GetServicesIndex(),
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
		servicelocator.GetServicesIndex(),
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

// stopArduinoContainers stops the Arduino containers that start running automatically when the board boots
func stopArduinoContainers(ctx context.Context, docker command.Cli) error {
	containers, err := docker.Client().ContainerList(ctx, container.ListOptions{
		All:     false,
		Filters: filters.NewArgs(filters.Arg("label", orchestrator.DockerAppLabel+"=true")),
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}
	for _, c := range containers {
		slog.Debug("Stopping container", slog.String("ID", c.ID))
		if err := docker.Client().ContainerStop(ctx, c.ID, container.StopOptions{}); err != nil {
			slog.Warn("Failed to stop container", "ID", c.ID, "error", err.Error())
		}
	}
	return nil
}
