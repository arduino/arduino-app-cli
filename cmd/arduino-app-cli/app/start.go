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

func newStartCmd(cfg config.Configuration) *cobra.Command {
	var verbose bool
	cmd := &cobra.Command{
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
			return startHandler(cmd.Context(), cfg, app, verbose)
		},
		ValidArgsFunction: completion.ApplicationNamesWithFilterFunc(cfg, func(apps orchestrator.AppInfo) bool {
			return apps.Status != orchestrator.StatusStarting &&
				apps.Status != orchestrator.StatusRunning
		}, servicelocator.GetPlatform()),
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	return cmd
}

func startHandler(ctx context.Context, cfg config.Configuration, app app.ArduinoApp, verbose bool) error {
	out, _, getResult := feedback.OutputStreams()

	err := orchestrator.StartApp(
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

func (r startAppResult) Data() any {
	return r
}
