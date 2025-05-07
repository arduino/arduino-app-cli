package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	dockerClient "github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"go.bug.st/cleanup"

	"github.com/arduino/arduino-app-cli/internal/api"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/pkg/httprecover"
	"github.com/arduino/arduino-app-cli/pkg/parser"
)

func main() {
	docker, err := dockerClient.NewClientWithOpts(
		dockerClient.FromEnv,
		dockerClient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		panic(err)
	}
	defer docker.Close()

	var daemonPort string
	var completionNoDesc bool // Disable completion description for shells that support it

	rootCmd := &cobra.Command{
		Use:   "arduino-app-cli",
		Short: "A CLI to manage the Python app",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
		},
	}

	completionCommand := &cobra.Command{
		Use:       "completion [bash|zsh|fish|powershell] [--no-descriptions]",
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Args:      cobra.ExactArgs(1),
		Short:     "Generates completion scripts",
		Long:      "Generates completion scripts for various shells",
		Example: "  " + os.Args[0] + " completion bash > completion.sh\n" +
			"  " + "source completion.sh",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletionV2(cmd.OutOrStdout(), !completionNoDesc)
			case "zsh":
				if completionNoDesc {
					return cmd.Root().GenZshCompletionNoDesc(cmd.OutOrStdout())
				} else {
					return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
				}
			case "fish":
				return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), !completionNoDesc)
			case "powershell":
				return cmd.Root().GenPowerShellCompletion(cmd.OutOrStdout())
			}
			return nil
		},
	}
	completionCommand.Flags().BoolVar(&completionNoDesc, "no-descriptions", false, "Disable completion description for shells that support it")

	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run an HTTP server to expose arduino-app-cli functionality thorough REST API",
		Run: func(cmd *cobra.Command, args []string) {
			httpHandler(cmd.Context(), docker, daemonPort)
		},
	}
	daemonCmd.Flags().StringVar(&daemonPort, "port", "8080", "The TCP port the daemon will listen to")

	rootCmd.AddCommand(
		&cobra.Command{
			Use:   "stop app_path",
			Short: "Stop the Python app",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				if len(args) == 0 {
					return cmd.Help()
				}
				app, err := parser.Load(args[0])
				if err != nil {
					return err
				}
				return stopHandler(cmd.Context(), app)
			},
		},
		&cobra.Command{
			Use:   "start app_path",
			Short: "Start the Python app",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				if len(args) == 0 {
					return cmd.Help()
				}
				app, err := parser.Load(args[0])
				if err != nil {
					return err
				}
				return startHandler(cmd.Context(), docker, app)
			},
		},
		&cobra.Command{
			Use:   "logs app_path",
			Short: "Show the logs of the Python app",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				if len(args) == 0 {
					return cmd.Help()
				}
				app, err := parser.Load(args[0])
				if err != nil {
					return err
				}
				return logsHandler(cmd.Context(), app)
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List all running Python apps",
			RunE: func(cmd *cobra.Command, args []string) error {
				return listHandler(cmd.Context())
			},
		},
		&cobra.Command{
			Use:   "provision app_path",
			Short: "Makes sure the Python app deps are downloaded and running",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				if len(args) == 0 {
					return cmd.Help()
				}
				app, err := parser.Load(args[0])
				if err != nil {
					return err
				}
				return provisionHandler(cmd.Context(), docker, app)
			},
		},
		completionCommand,
		daemonCmd,
	)

	ctx := context.Background()
	ctx, _ = cleanup.InterruptableContext(ctx)
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		slog.Error(err.Error())
	}
}

func provisionHandler(ctx context.Context, docker *dockerClient.Client, app parser.App) error {
	if err := orchestrator.ProvisionApp(ctx, docker, app); err != nil {
		return err
	}
	return nil
}

func startHandler(ctx context.Context, docker *dockerClient.Client, app parser.App) error {
	for message := range orchestrator.StartApp(ctx, docker, app) {
		switch message.GetType() {
		case orchestrator.ProgressType:
			slog.Info("progress", slog.Float64("progress", float64(message.GetProgress().Progress)))
		case orchestrator.InfoType:
			slog.Info("log", slog.String("message", message.GetData()))
		case orchestrator.ErrorType:
			return errors.New(message.GetError().Error())
		}
	}
	return nil
}

func stopHandler(ctx context.Context, app parser.App) error {
	for message := range orchestrator.StopApp(ctx, app) {
		switch message.GetType() {
		case orchestrator.ProgressType:
			slog.Info("progress", slog.Float64("progress", float64(message.GetProgress().Progress)))
		case orchestrator.InfoType:
			slog.Info("log", slog.String("message", message.GetData()))
		case orchestrator.ErrorType:
			return errors.New(message.GetError().Error())
		}
	}
	return nil
}

func logsHandler(ctx context.Context, app parser.App) error {
	logsIter, err := orchestrator.AppLogs(ctx, app, orchestrator.AppLogsRequest{ShowAppLogs: true, Follow: true})
	if err != nil {
		return err
	}
	for msg := range logsIter {
		fmt.Printf("[%s] %s\n", msg.Name, msg.Content)
	}
	return nil
}

func listHandler(ctx context.Context) error {
	res, err := orchestrator.ListApps(ctx)
	if err != nil {
		return nil
	}

	resJSON, err := json.Marshal(res)
	if err != nil {
		return nil
	}
	fmt.Println(string(resJSON))
	return nil
}

func httpHandler(ctx context.Context, dockerClient *dockerClient.Client, daemonPort string) {
	slog.Info("Starting HTTP server", slog.String("address", ":"+daemonPort))
	apiSrv := api.NewHTTPRouter(dockerClient)

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
