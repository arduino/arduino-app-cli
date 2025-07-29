package app

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

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
			app, err := Load(args[0])
			if err != nil {
				return err
			}
			return logsHandler(cmd.Context(), app, &tail)
		},
		ValidArgsFunction: completion.ApplicationNames(),
	}
	cmd.Flags().Uint64Var(&tail, "tail", 100, "Tail the last N logs")
	return cmd
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
