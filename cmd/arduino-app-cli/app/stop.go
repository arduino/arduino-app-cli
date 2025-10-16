// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package app

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newStopCmd(cfg config.Configuration) *cobra.Command {
	return &cobra.Command{
		Use:   "stop app_path",
		Short: "Stop an Arduino App",
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
		ValidArgsFunction: completion.ApplicationNamesWithFilterFunc(cfg, func(apps orchestrator.AppInfo) bool {
			return apps.Status == orchestrator.StatusStarting ||
				apps.Status == orchestrator.StatusRunning
		}),
	}
}

func stopHandler(ctx context.Context, app app.ArduinoApp) error {
	out, _, getResult := feedback.OutputStreams()

	for message := range orchestrator.StopApp(ctx, app) {
		switch message.GetType() {
		case orchestrator.ProgressType:
			fmt.Fprintf(out, "Progress[%s]: %.0f%%\n", message.GetProgress().Name, message.GetProgress().Progress)
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
