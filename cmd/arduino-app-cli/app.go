package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/arduino/go-paths-helper"
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/cmd/results"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

func newAppCmd() *cobra.Command {
	appCmd := &cobra.Command{
		Use:   "app",
		Short: "Manage Arduino Apps",
		Long:  "A CLI tool to manage Arduino Apps, including starting, stopping, logging, and provisioning.",
	}

	appCmd.AddCommand(newCreateCmd())
	appCmd.AddCommand(newStartCmd())
	appCmd.AddCommand(newStopCmd())
	appCmd.AddCommand(newRestartCmd())
	appCmd.AddCommand(newLogsCmd())
	appCmd.AddCommand(newListCmd())
	appCmd.AddCommand(newPsCmd())
	appCmd.AddCommand(newMonitorCmd())

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

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start app_path",
		Short: "Start an Arduino app",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			app, err := loadApp(args[0])
			if err != nil {
				return err
			}
			return startHandler(cmd.Context(), app)
		},
		ValidArgsFunction: ApplicationNames(),
	}
}

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop app_path",
		Short: "Stop an Arduino app",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			app, err := loadApp(args[0])
			if err != nil {
				return err
			}
			return stopHandler(cmd.Context(), app)
		},
		ValidArgsFunction: ApplicationNames(),
	}
}

func newRestartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart app_path",
		Short: "Restart or Start an Arduino app",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			app, err := loadApp(args[0])
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrBadArgument)
				return nil
			}
			if err := stopHandler(cmd.Context(), app); err != nil {
				feedback.Warning(fmt.Sprintf("failed to stop app: %s", err.Error()))
			}
			return startHandler(cmd.Context(), app)
		},
		ValidArgsFunction: ApplicationNames(),
	}
	return cmd
}

func newLogsCmd() *cobra.Command {
	var tail uint64
	cmd := &cobra.Command{
		Use:   "logs app_path",
		Short: "Show the logs of the Python app",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			app, err := loadApp(args[0])
			if err != nil {
				return err
			}
			return logsHandler(cmd.Context(), app, &tail)
		},
		ValidArgsFunction: ApplicationNames(),
	}
	cmd.Flags().Uint64Var(&tail, "tail", 100, "Tail the last N logs")
	return cmd
}

func newMonitorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "monitor",
		Short: "Monitor the Python app",
		RunE: func(cmd *cobra.Command, args []string) error {
			panic("not implemented")
		},
		ValidArgsFunction: ApplicationNames(),
	}

}

func newListCmd() *cobra.Command {
	var jsonFormat bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all running Python apps",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listHandler(cmd.Context())
		},
	}

	cmd.Flags().BoolVarP(&jsonFormat, "json", "", false, "Output the list in json format")
	return cmd
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

func newPropertiesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "properties",
		Short: "Manage apps properties",
		Long:  "Manage apps properties, including setting and getting the default app.",
	}

	cmd.AddCommand(&cobra.Command{
		Use:       "get default",
		Short:     "Get properties, e.g., default",
		ValidArgs: []string{"default"},
		Args:      cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			def, err := orchestrator.GetDefaultApp()
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrGeneric)
			}
			feedback.PrintResult(results.DefaultAppResult{
				App: def,
			})
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:       "set default <app_path>",
		Short:     "Set properties, e.g., default",
		Long:      "Set properties. Use 'none' to unset a property.",
		ValidArgs: []string{"default"},
		Args:      cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			// Remove default app.
			if len(args) == 1 || args[1] == "none" {
				if err := orchestrator.SetDefaultApp(nil); err != nil {
					feedback.Fatal(err.Error(), feedback.ErrGeneric)
					return nil
				}
				feedback.PrintResult(results.DefaultAppResult{App: nil})
				return nil
			}

			app, err := loadApp(args[1])
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrBadArgument)
				return nil
			}
			if err := orchestrator.SetDefaultApp(&app); err != nil {
				feedback.Fatal(err.Error(), feedback.ErrGeneric)
				return nil
			}
			feedback.PrintResult(results.DefaultAppResult{App: &app})
			return nil
		},
	})

	return cmd
}

func startHandler(ctx context.Context, app app.ArduinoApp) error {
	out, _, getResult := feedback.OutputStreams()

	stream := orchestrator.StartApp(
		ctx,
		servicelocator.GetDockerClient(),
		servicelocator.GetProvisioner(),
		servicelocator.GetModelsIndex(),
		servicelocator.GetBricksIndex(),
		app,
	)
	for message := range stream {
		switch message.GetType() {
		case orchestrator.ProgressType:
			fmt.Fprintf(out, "Progress: %.0f%%\n", message.GetProgress().Progress)
		case orchestrator.InfoType:
			fmt.Fprintln(out, "[INFO]", message.GetData())
		case orchestrator.ErrorType:
			err := errors.New(message.GetError().Error())
			feedback.Fatal(err.Error(), feedback.ErrGeneric)
			return nil
		}
	}
	outputResult := getResult()
	feedback.PrintResult(results.StartAppResult{
		AppName: app.Name,
		Status:  "started",
		Output:  outputResult,
	})

	return nil
}

func stopHandler(ctx context.Context, app app.ArduinoApp) error {
	out, _, getResult := feedback.OutputStreams()

	for message := range orchestrator.StopApp(ctx, app) {
		switch message.GetType() {
		case orchestrator.ProgressType:
			fmt.Fprintf(out, "Progress: %.0f%%\n", message.GetProgress().Progress)
		case orchestrator.InfoType:
			fmt.Fprintln(out, "[INFO]", message.GetData())
		case orchestrator.ErrorType:
			err := errors.New(message.GetError().Error())
			feedback.Fatal(err.Error(), feedback.ErrGeneric)
			return nil
		}
	}
	outputResult := getResult()

	feedback.PrintResult(results.StopAppResult{
		AppName: app.Name,
		Status:  "stopped",
		Output:  outputResult,
	})
	return nil
}

func logsHandler(ctx context.Context, app app.ArduinoApp, tail *uint64) error {
	stdout, _, err := feedback.DirectStreams()
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrBadArgument)
		return nil
	}

	logsIter, err := orchestrator.AppLogs(
		ctx,
		app,
		orchestrator.AppLogsRequest{
			ShowAppLogs: true,
			Follow:      true,
			Tail:        tail,
		},
	)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
		return nil
	}
	for msg := range logsIter {
		fmt.Fprintf(stdout, "[%s] %s\n", msg.Name, msg.Content)
	}
	return nil
}

func listHandler(ctx context.Context) error {
	res, err := orchestrator.ListApps(ctx,
		servicelocator.GetDockerClient(),
		orchestrator.ListAppRequest{
			ShowExamples:                   true,
			ShowApps:                       true,
			IncludeNonStandardLocationApps: true,
		},
	)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
		return nil
	}

	feedback.PrintResult(results.AppListResult{
		Apps:       res.Apps,
		BrokenApps: res.BrokenApps,
	})

	return nil
}

func createHandler(ctx context.Context, name string, icon string, bricks []string, noPython, noSketch bool, fromApp string) error {
	if fromApp != "" {
		wd, err := paths.Getwd()
		if err != nil {
			feedback.Fatal(err.Error(), feedback.ErrGeneric)
			return nil
		}
		fromPath := paths.New(fromApp)
		if !fromPath.IsAbs() {
			fromPath = wd.JoinPath(fromPath)
		}
		id, err := orchestrator.NewIDFromPath(fromPath)
		if err != nil {
			feedback.Fatal(err.Error(), feedback.ErrBadArgument)
			return nil
		}

		resp, err := orchestrator.CloneApp(ctx, orchestrator.CloneAppRequest{
			Name:   &name,
			FromID: id,
		})
		if err != nil {
			feedback.Fatal(err.Error(), feedback.ErrGeneric)
			return nil
		}
		dst := resp.ID.ToPath()

		feedback.PrintResult(results.CreateAppResult{
			Result:  "ok",
			Message: "App created successfully",
			Path:    dst.String(),
		})

	} else {
		resp, err := orchestrator.CreateApp(ctx, orchestrator.CreateAppRequest{
			Name:       name,
			Icon:       icon,
			Bricks:     bricks,
			SkipPython: noPython,
			SkipSketch: noSketch,
		})
		if err != nil {
			feedback.Fatal(err.Error(), feedback.ErrGeneric)
			return nil
		}
		feedback.PrintResult(results.CreateAppResult{
			Result:  "ok",
			Message: "App created successfully",
			Path:    resp.ID.ToPath().String(),
		})
	}
	return nil
}

func loadApp(idOrPath string) (app.ArduinoApp, error) {
	id, err := orchestrator.ParseID(idOrPath)
	if err != nil {
		return app.ArduinoApp{}, fmt.Errorf("invalid app path: %s", idOrPath)
	}

	return app.Load(id.ToPath().String())
}
