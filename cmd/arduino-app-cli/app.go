package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/pkg/parser"

	"github.com/arduino/go-paths-helper"
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
	appCmd.AddCommand(newSetDefaultCmd())
	appCmd.AddCommand(newGetDefaultCmd())

	return appCmd
}

func newCreateCmd() *cobra.Command {
	var (
		icon     string
		bricks   []string
		noPyton  bool
		noSketch bool
		fromApp  string
	)

	cmd := &cobra.Command{
		Use:   "new name",
		Short: "Creates a new app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cobra.MinimumNArgs(1)
			name := args[0]
			return createHandler(cmd.Context(), name, icon, bricks, noPyton, noSketch, fromApp)
		},
	}

	cmd.Flags().StringVarP(&icon, "icon", "i", "", "Icon for the app")
	cmd.Flags().StringVarP(&fromApp, "from-app", "", "", "Create the new app from the path of an existing app")
	cmd.Flags().StringArrayVarP(&bricks, "bricks", "b", []string{}, "List of bricks to include in the app")
	cmd.Flags().BoolVarP(&noPyton, "no-python", "", false, "Do not include Python files")
	cmd.Flags().BoolVarP(&noSketch, "no-sketch", "", false, "Do not include Sketch files")
	cmd.MarkFlagsMutuallyExclusive("no-python", "no-sketch")

	return cmd
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

func renderDefaultApp(app *parser.App) {
	if app == nil {
		fmt.Println("No default app set")
	} else {
		fmt.Printf("Default app: %s (%s)\n", app.Name, app.FullPath)
	}
}

func newSetDefaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-default app_path",
		Short: "Set the default app",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Remove default app.
			if len(args) == 0 {
				return orchestrator.SetDefaultApp(nil)
			}

			app, err := parser.Load(args[0])
			if err != nil {
				return err
			}
			if err := orchestrator.SetDefaultApp(&app); err != nil {
				return err
			}
			renderDefaultApp(&app)
			return nil
		},
	}
}

func newGetDefaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get-default",
		Short: "Get the default app",
		Args:  cobra.MaximumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			def, err := orchestrator.GetDefaultApp()
			if err != nil {
				return err
			}
			renderDefaultApp(def)
			return nil
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

func createHandler(ctx context.Context, name string, icon string, bricks []string, noPython, noSketch bool, fromApp string) error {
	if fromApp != "" {
		wd, err := paths.Getwd()
		if err != nil {
			return err
		}
		fromPath := paths.New(fromApp)
		if !fromPath.IsAbs() {
			fromPath = wd.JoinPath(fromPath)
		}
		id, err := orchestrator.NewIDFromPath(fromPath)
		if err != nil {
			return err
		}

		resp, err := orchestrator.CloneApp(ctx, orchestrator.CloneAppRequest{
			Name:   &name,
			FromID: id,
		})
		if err != nil {
			return err
		}
		dst, _ := resp.ID.ToPath()
		fmt.Println("App cloned in: ", dst)
	} else {
		resp, err := orchestrator.CreateApp(ctx, orchestrator.CreateAppRequest{
			Name:       name,
			Icon:       icon,
			Bricks:     bricks,
			SkipPython: noPython,
			SkipSketch: noSketch,
		})
		if err != nil {
			return err
		}
		fmt.Println("App created successfully:", resp)
	}
	return nil
}
