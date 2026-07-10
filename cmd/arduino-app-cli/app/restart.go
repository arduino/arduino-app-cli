// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

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
	var verbose bool
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
			return restartHandler(cmd.Context(), cfg, appToStart, verbose)
		},
		ValidArgsFunction: completion.ApplicationNames(cfg),
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	return cmd
}

func restartHandler(ctx context.Context, cfg config.Configuration, app app.ArduinoApp, verbose bool) error {
	out, _, getResult := feedback.OutputStreams()

	err := orchestrator.RestartApp(
		ctx,
		servicelocator.GetDockerClient(),
		servicelocator.GetProvisioner(),
		servicelocator.GetModelsIndex(),
		servicelocator.GetBricksIndex(),
		servicelocator.GetServicesIndex(),
		app,
		cfg,
		servicelocator.GetPlatform(),
		verbose,
		func(message orchestrator.StreamMessage) {
			switch message.GetType() {
			case orchestrator.ProgressType:
				fmt.Fprintf(out, "Progress[%s]: %.0f%%\n", message.GetProgress().Name, message.GetProgress().Progress)
			case orchestrator.InfoType:
				fmt.Fprintln(out, "[INFO]", message.GetData())
			}
		},
	)
	if err != nil {
		errMesg := cases.Title(language.AmericanEnglish).String(err.Error())
		feedback.Fatal(fmt.Sprintf("[ERROR] %s", errMesg), feedback.ErrGeneric)
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

func (r restartAppResult) Data() any {
	return r
}
