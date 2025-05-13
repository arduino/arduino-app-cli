package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/pkg/parser"

	dockerClient "github.com/docker/docker/client"
	"github.com/spf13/cobra"
)

func newAppCmd(docker *dockerClient.Client) *cobra.Command {
	appCmd := &cobra.Command{
		Use:   "app",
		Short: "Manage Arduino Apps",
		Long:  "A CLI tool to manage Arduino Apps, including starting, stopping, logging, and provisioning.",
	}

	appCmd.AddCommand(newCreateCmd())
	appCmd.AddCommand(newStartCmd(docker))
	appCmd.AddCommand(newStopCmd())
	appCmd.AddCommand(newLogsCmd())
	appCmd.AddCommand(newListCmd())
	appCmd.AddCommand(newPsCmd())
	appCmd.AddCommand(newProvisionCmd(docker))
	appCmd.AddCommand(newMonitorCmd())

	return appCmd
}

func newCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new name",
		Short: "Creates a new app",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			panic("not implemented")
		},
	}
}

func newStartCmd(docker *dockerClient.Client) *cobra.Command {
	return &cobra.Command{
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
	}
}

func newStopCmd() *cobra.Command {
	return &cobra.Command{
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
	}
}

func newLogsCmd() *cobra.Command {
	return &cobra.Command{
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
	}
}

func newMonitorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "monitor",
		Short: "Monitor the Python app",
		RunE: func(cmd *cobra.Command, args []string) error {
			panic("not implemented")
		},
	}

}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all running Python apps",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listHandler(cmd.Context())
		},
	}
}

func newPsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ps",
		Short: "Shows the list of running Arduino Apps",
		RunE: func(cmd *cobra.Command, args []string) error {
			panic("not implemented")
		},
	}
}

func newProvisionCmd(docker *dockerClient.Client) *cobra.Command {
	return &cobra.Command{
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
