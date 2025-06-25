package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/arduino/arduino-app-cli/internal/api"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/pkg/httprecover"

	dockerClient "github.com/docker/docker/client"
	"github.com/jub0bs/cors"
	"github.com/spf13/cobra"
)

func newDaemonCmd(docker *dockerClient.Client) *cobra.Command {
	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run an HTTP server to expose arduino-app-cli functionality thorough REST API",
		Run: func(cmd *cobra.Command, args []string) {
			daemonPort, _ := cmd.Flags().GetString("port")

			// start the default app in the background
			go func() {
				slog.Info("Starting default app")
				err := orchestrator.StartDefaultApp(cmd.Context(), docker)
				if err != nil {
					slog.Error("Failed to start default app", slog.String("error", err.Error()))
				}
				slog.Info("Default app started")
			}()

			httpHandler(cmd.Context(), docker, daemonPort)
		},
	}
	daemonCmd.Flags().String("port", "8080", "The TCP port the daemon will listen to")
	return daemonCmd
}

func httpHandler(ctx context.Context, dockerClient *dockerClient.Client, daemonPort string) {
	slog.Info("Starting HTTP server", slog.String("address", ":"+daemonPort))
	apiSrv := api.NewHTTPRouter(dockerClient, Version)

	corsMiddlware, err := cors.NewMiddleware(
		cors.Config{
			Origins: []string{
				"wails://wails.localhost:34115",
				"http://wails.localhost:34115",
				"http://localhost:*", "https://localhost:*",
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
			},
			MaxAgeInSeconds: 86400,
			ResponseHeaders: []string{},
		},
	)
	if err != nil {
		panic(err)
	}

	httpSrv := http.Server{
		Addr:              ":" + daemonPort,
		Handler:           httprecover.RecoverPanic(corsMiddlware.Wrap(apiSrv)),
		ReadHeaderTimeout: 60 * time.Second,
	}
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(err.Error())
		}
	}()

	<-ctx.Done()
	slog.Info("Shutting down HTTP server", slog.String("address", ":"+daemonPort))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	_ = httpSrv.Shutdown(ctx)
	cancel()
	slog.Info("HTTP server shut down", slog.String("address", ":"+daemonPort))
}
