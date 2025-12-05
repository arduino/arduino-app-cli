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
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newStartCmd(cfg config.Configuration) *cobra.Command {
	return &cobra.Command{
		Use:   "start app_path",
		Short: "Start an Arduino App",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			app, err := Load(args[0])
			if err != nil {
				return err
			}
			return startHandler(cmd.Context(), cfg, app)
		},
		ValidArgsFunction: completion.ApplicationNamesWithFilterFunc(cfg, func(apps orchestrator.AppInfo) bool {
			return apps.Status != orchestrator.StatusStarting &&
				apps.Status != orchestrator.StatusRunning
		}),
	}
}

func startHandler(ctx context.Context, cfg config.Configuration, app app.ArduinoApp) error {
	out, _, getResult := feedback.OutputStreams()

	stream := orchestrator.StartApp(
		ctx,
		servicelocator.GetDockerClient(),
		servicelocator.GetProvisioner(),
		servicelocator.GetModelsIndex(),
		servicelocator.GetBricksIndex(),
		app,
		cfg,
		servicelocator.GetStaticStore(),
	)
	for message := range stream {
		switch message.GetType() {
		case orchestrator.ProgressType:
			fmt.Fprintf(out, "Progress[%s]: %.0f%%\n", message.GetProgress().Name, message.GetProgress().Progress)
		case orchestrator.InfoType:
			fmt.Fprintln(out, "[INFO]", message.GetData())
		case orchestrator.ErrorType:
			errMesg := cases.Title(language.AmericanEnglish).String(message.GetError().Error())
			feedback.Fatal(fmt.Sprintf("[ERROR] %s", errMesg), feedback.ErrGeneric)
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
