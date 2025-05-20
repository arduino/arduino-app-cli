package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/arduino/arduino-app-cli/internal/api"
	"github.com/arduino/arduino-app-cli/pkg/httprecover"

	dockerClient "github.com/docker/docker/client"
	"github.com/spf13/cobra"
)

func newDaemonCmd(docker *dockerClient.Client) *cobra.Command {
	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run an HTTP server to expose arduino-app-cli functionality thorough REST API",
		Run: func(cmd *cobra.Command, args []string) {
			daemonPort, _ := cmd.Flags().GetString("port")
			httpHandler(cmd.Context(), docker, daemonPort)
		},
	}
	daemonCmd.Flags().String("port", "8080", "The TCP port the daemon will listen to")
	return daemonCmd
}

func httpHandler(ctx context.Context, dockerClient *dockerClient.Client, daemonPort string) {
	slog.Info("Starting HTTP server", slog.String("address", ":"+daemonPort))
	apiSrv := api.NewHTTPRouter(dockerClient, Version)

	httpSrv := http.Server{
		Addr:              ":" + daemonPort,
		Handler:           httprecover.RecoverPanic(apiSrv),
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
