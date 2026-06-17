// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

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
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newDestroyCmd(cfg config.Configuration) *cobra.Command {
	return &cobra.Command{
		Use:   "destroy app_path",
		Short: "Destroy an Arduino App",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			app, err := Load(args[0])
			if err != nil {
				return err
			}
			return destroyHandler(cmd.Context(), app)
		},
		ValidArgsFunction: completion.ApplicationNamesWithFilterFunc(cfg, func(apps orchestrator.AppInfo) bool {
			return apps.Status != orchestrator.StatusUninitialized
		}, servicelocator.GetPlatform()),
	}
}

func destroyHandler(ctx context.Context, app app.ArduinoApp) error {
	out, _, getResult := feedback.OutputStreams()

	if err := orchestrator.StopAndDestroyApp(ctx, servicelocator.GetDockerClient(), servicelocator.GetPlatform(), app, func(message orchestrator.StreamMessage) {
		switch message.GetType() {
		case orchestrator.ProgressType:
			fmt.Fprintf(out, "Progress[%s]: %.0f%%\n", message.GetProgress().Name, message.GetProgress().Progress)
		case orchestrator.InfoType:
			fmt.Fprintln(out, "[INFO]", message.GetData())
		}
	}); err != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	}
	outputResult := getResult()

	feedback.PrintResult(destroyAppResult{
		AppName: app.Name,
		Status:  "uninitialized",
		Output:  outputResult,
	})
	return nil
}

type destroyAppResult struct {
	AppName string                        `json:"appName"`
	Status  string                        `json:"status"`
	Output  *feedback.OutputStreamsResult `json:"output,omitempty"`
}

func (r destroyAppResult) String() string {
	return fmt.Sprintf("✓ App '%q destroyed successfully.", r.AppName)
}

func (r destroyAppResult) Data() any {
	return r
}
