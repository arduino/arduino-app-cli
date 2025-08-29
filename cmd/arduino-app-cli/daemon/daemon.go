package daemon

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/internal/api"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/update"
	"github.com/arduino/arduino-app-cli/internal/update/apt"
	"github.com/arduino/arduino-app-cli/internal/update/arduino"
	"github.com/arduino/arduino-app-cli/pkg/httprecover"

	"github.com/jub0bs/cors"
	"github.com/spf13/cobra"
)

func NewDaemonCmd(cfg config.Configuration, version string) *cobra.Command {
	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run an HTTP server to expose arduino-app-cli functionality thorough REST API",
		Run: func(cmd *cobra.Command, args []string) {
			daemonPort, _ := cmd.Flags().GetString("port")

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
				)
				if err != nil {
					slog.Error("Failed to start default app", slog.String("error", err.Error()))
				} else {
					slog.Info("Default app started")
				}

				slog.Info("Try to pull latest docker images in background...")
				err = orchestrator.SystemInit(
					cmd.Context(),
					cfg,
					servicelocator.GetStaticStore(),
				)
				if err != nil {
					slog.Error("Auto-pull process failed", slog.String("error", err.Error()))
				} else {
					slog.Info("Auto-pull process completed.")
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

	apiSrv := api.NewHTTPRouter(
		servicelocator.GetDockerClient(),
		version,
		update.NewManager(
			apt.New(),
			arduino.NewArduinoPlatformUpdater(),
		),
		servicelocator.GetProvisioner(),
		servicelocator.GetStaticStore(),
		servicelocator.GetModelsIndex(),
		servicelocator.GetBricksIndex(),
		servicelocator.GetBrickService(),
		servicelocator.GetAppIDProvider(),
		cfg,
	)

	corsMiddlware, err := cors.NewMiddleware(
		cors.Config{
			Origins: []string{
				"wails://wails", "wails://wails.localhost:*",
				"http://wails.localhost", "http://wails.localhost:*",
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

	address := "127.0.0.1:" + daemonPort
	httpSrv := http.Server{
		Addr:              address,
		Handler:           httprecover.RecoverPanic(corsMiddlware.Wrap(apiSrv)),
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
