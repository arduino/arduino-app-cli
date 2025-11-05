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

func newRestartCmd(cfg config.Configuration) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart app_path",
		Short: "Restart or Start an Arduino App",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			appToStart, err := Load(args[0])
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrBadArgument)
			}
			return restartHandler(cmd.Context(), cfg, appToStart)
		},
		ValidArgsFunction: completion.ApplicationNames(cfg),
	}
	return cmd
}

func restartHandler(ctx context.Context, cfg config.Configuration, app app.ArduinoApp) error {
	out, _, getResult := feedback.OutputStreams()

	stream := orchestrator.RestartApp(
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
	feedback.PrintResult(restartAppResult{
		AppName: app.Name,
		Status:  "restarted",
		Output:  outputResult,
	})

	return nil
}

type restartAppResult struct {
	AppName string                        `json:"app_name"`
	Status  string                        `json:"status"`
	Output  *feedback.OutputStreamsResult `json:"output,omitempty"`
}

func (r restartAppResult) String() string {
	return fmt.Sprintf("✓ App %q restarted successfully", r.AppName)
}

func (r restartAppResult) Data() interface{} {
	return r
}
