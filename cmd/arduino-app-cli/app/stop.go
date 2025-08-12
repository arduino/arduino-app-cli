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

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop app_path",
		Short: "Stop an Arduino app",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			app, err := Load(args[0])
			if err != nil {
				return err
			}
			return stopHandler(cmd.Context(), app)
		},
		ValidArgsFunction: completion.ApplicationNames(),
	}
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
			feedback.Fatal(message.GetError().Error(), feedback.ErrGeneric)
			return nil
		}
	}
	outputResult := getResult()

	feedback.PrintResult(stopAppResult{
		AppName: app.Name,
		Status:  "stopped",
		Output:  outputResult,
	})
	return nil
}

type stopAppResult struct {
	AppName string                        `json:"appName"`
	Status  string                        `json:"status"`
	Output  *feedback.OutputStreamsResult `json:"output,omitempty"`
}

func (r stopAppResult) String() string {
	return fmt.Sprintf("✓ App '%q stopped successfully.", r.AppName)
}

func (r stopAppResult) Data() interface{} {
	return r
}
