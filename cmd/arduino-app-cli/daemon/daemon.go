// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package daemon

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/docker/cli/cli/command"
	"github.com/jub0bs/cors"
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/internal/api"
	"github.com/arduino/arduino-app-cli/internal/dockerhelper"
	"github.com/arduino/arduino-app-cli/internal/httprecover"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/pipewire"
	"github.com/arduino/arduino-app-cli/internal/releasebuild"
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

			app, err := orchestrator.GetDefaultApp(cfg)
			if err != nil {
				slog.Warn("failed to get default app", slog.String("error", err.Error()))
			}
			if app == nil {
				slog.Debug("app is not set")
				if err := pipewire.StopIfNotNeeded(cmd.Context(), cfg); err != nil {
					slog.Warn("Failed to reconcile leftover audio service linger", slog.String("error", err.Error()))
				}
			}

			// start the default app in the background
			go startDefaultApp(cmd.Context(), func() error {
				return orchestrator.StartDefaultApp(
					cmd.Context(),
					servicelocator.GetDockerClient(),
					servicelocator.GetProvisioner(),
					servicelocator.GetModelsIndex(),
					servicelocator.GetBricksIndex(),
					servicelocator.GetServicesIndex(),
					servicelocator.GetAppIDProvider(),
					cfg,
					servicelocator.GetPlatform(),
				)
			})

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
		).WithSelfRestart(),
		servicelocator.GetProvisioner(),
		servicelocator.GetModelsIndex(),
		servicelocator.GetBricksIndex(),
		servicelocator.GetServicesIndex(),
		servicelocator.GetBrickService(),
		servicelocator.GetAppIDProvider(),
		servicelocator.GetPlatform(),
		cfg,
		releasebuild.NewEventBroker(),
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
	// All the requests derive from srvCtx: canceling it interrupts all requests, including the long-lived
	// SSE streams, that otherwise block the shutdown until the timeout.
	srvCtx, cancelRequests := context.WithCancel(context.Background())
	httpSrv := http.Server{
		Addr:              address,
		Handler:           httprecover.RecoverPanic(apiSrv),
		ReadHeaderTimeout: 60 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return srvCtx },
	}
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(err.Error())
		}
	}()

	<-ctx.Done()
	slog.Info("Shutting down HTTP server", slog.String("address", address))

	cancelRequests()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	_ = httpSrv.Shutdown(shutdownCtx)
	cancel()
	slog.Info("HTTP server shut down", slog.String("address", address))
}

// stopArduinoContainers stops the Arduino containers that start running automatically when the board boots
func stopArduinoContainers(ctx context.Context, docker command.Cli) error {
	stopped, err := dockerhelper.StopContainers(ctx, docker.Client(), orchestrator.DockerAppLabel+"=true")
	slog.Debug("stopped the containers of the apps", slog.Int("containers", stopped))
	return err
}

// startDefaultApp starts the default app, and starts it again when a start fails. The
// board is usually powered off by disconnecting it, so at boot the network may not be
// up yet and the docker engine may still be removing what the power loss left.
// 10s, 20s, 40s, 80s, 160s, then give up.
func startDefaultApp(ctx context.Context, start func() error) {
	const (
		minBackoff = 10 * time.Second
		maxBackoff = 160 * time.Second
	)
	slog.Info("Starting default app")
	for backoff := minBackoff; ; backoff *= 2 {
		err := start()
		if err == nil {
			slog.Info("Default app started")
			return
		}

		// The daemon is stopping: the start failed for that.
		if ctx.Err() != nil {
			return
		}

		// An app started meanwhile is the one the user wants: it is not stopped for this.
		if errors.Is(err, orchestrator.ErrAppRunning) {
			slog.Info("Default app not started", slog.String("error", err.Error()))
			return
		}
		if backoff > maxBackoff {
			slog.Error("Failed to start default app", slog.String("error", err.Error()))
			return
		}
		slog.Error("Failed to start default app, trying again", slog.String("error", err.Error()), slog.Duration("delay", backoff))

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
	}
}
