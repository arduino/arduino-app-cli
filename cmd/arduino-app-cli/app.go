package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/arduino/go-paths-helper"
	dockerClient "github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"mkuznets.com/go/tabwriter"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
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
	appCmd.AddCommand(newListCmd(docker))
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

func newStartCmd(docker *dockerClient.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "start app_path",
		Short: "Start the Python app",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			app, err := loadApp(args[0])
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
			app, err := loadApp(args[0])
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
			app, err := loadApp(args[0])
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

func newListCmd(docker *dockerClient.Client) *cobra.Command {
	var jsonFormat bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all running Python apps",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listHandler(cmd.Context(), docker, jsonFormat)
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

func renderDefaultApp(app *app.ArduinoApp) {
	if app == nil {
		fmt.Println("No default app set")
	} else {
		fmt.Printf("Default app: %s (%s)\n", app.Name, app.FullPath)
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
				return err
			}
			renderDefaultApp(def)
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
				return orchestrator.SetDefaultApp(nil)
			}

			app, err := loadApp(args[1])
			if err != nil {
				return err
			}
			if err := orchestrator.SetDefaultApp(&app); err != nil {
				return err
			}
			renderDefaultApp(&app)
			return nil
		},
	})

	return cmd
}

func startHandler(ctx context.Context, docker *dockerClient.Client, app app.ArduinoApp) error {
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

func stopHandler(ctx context.Context, app app.ArduinoApp) error {
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

func logsHandler(ctx context.Context, app app.ArduinoApp) error {
	logsIter, err := orchestrator.AppLogs(ctx, app, orchestrator.AppLogsRequest{ShowAppLogs: true, Follow: true})
	if err != nil {
		return err
	}
	for msg := range logsIter {
		fmt.Printf("[%s] %s\n", msg.Name, msg.Content)
	}
	return nil
}

func listHandler(ctx context.Context, docker *dockerClient.Client, jsonFormat bool) error {
	res, err := orchestrator.ListApps(ctx, docker, orchestrator.ListAppRequest{
		ShowExamples:                   true,
		ShowApps:                       true,
		IncludeNonStandardLocationApps: true,
	})
	if err != nil {
		return nil
	}

	idToAlias := func(id orchestrator.ID) string {
		v := id.String()
		res, err := base64.RawURLEncoding.DecodeString(v)
		if err != nil {
			return v
		}

		v = string(res)
		if strings.Contains(v, ":") {
			return v
		}

		wd, err := paths.Getwd()
		if err != nil {
			return v
		}
		rel, err := paths.New(v).RelFrom(wd)
		if err != nil {
			return v
		}
		if !strings.HasPrefix(rel.String(), "./") && !strings.HasPrefix(rel.String(), "../") {
			return "./" + rel.String()
		}
		return rel.String()
	}

	if jsonFormat {
		// Print in JSON format.
		resJSON, err := json.Marshal(res)
		if err != nil {
			return nil
		}
		fmt.Println(string(resJSON))
	} else {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0) // minwidth, tabwidth, padding, padchar, flags
		fmt.Fprintln(w, "ID\tNAME\tICON\tSTATUS\tEXAMPLE")

		for _, app := range res.Apps {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%t\n",
				idToAlias(app.ID),
				app.Name,
				app.Icon,
				app.Status,
				app.Example,
			)
		}

		if len(res.BrokenApps) > 0 {
			fmt.Fprintln(w, "\nAPP\tERROR")
			for _, app := range res.BrokenApps {
				fmt.Fprintf(w, "%s\t%s\n",
					app.Name,
					app.Error,
				)
			}
		}
		w.Flush()
	}

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
		dst := resp.ID.ToPath()
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
		fmt.Println("App created successfully:", resp.ID.ToPath().String())
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
