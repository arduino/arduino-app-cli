// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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
		servicelocator.GetServicesIndex(),
		app,
		cfg,
		servicelocator.GetStaticStore(),
		servicelocator.GetPlatform(),
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
