package app

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start app_path",
		Short: "Start an Arduino app",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			app, err := Load(args[0])
			if err != nil {
				return err
			}
			return startHandler(cmd.Context(), app)
		},
		ValidArgsFunction: completion.ApplicationNames(),
	}
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
			feedback.Fatal(message.GetError().Error(), feedback.ErrGeneric)
			return nil
		}
	}
	outputResult := getResult()
	feedback.PrintResult(startAppResult{
		AppName: app.Name,
		Status:  "started",
		Output:  outputResult,
	})

	return nil
}

type startAppResult struct {
	AppName string                        `json:"appName"`
	Status  string                        `json:"status"`
	Output  *feedback.OutputStreamsResult `json:"output,omitempty"`
}

func (r startAppResult) String() string {
	return fmt.Sprintf("✓ App %q started successfully", r.AppName)
}

func (r startAppResult) Data() interface{} {
	return r
}
