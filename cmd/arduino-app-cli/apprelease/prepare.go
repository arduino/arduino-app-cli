// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package apprelease

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newPrepareCmd(cfg config.Configuration) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prepare app_path",
		Short: "Pre-pull the container images an installed app needs",
		Long: `Pre-pull the Docker images referenced by an installed app's compose file so that a
subsequent 'app start' finds them locally and does not need network access.

The app is not started, and no AI models are downloaded (model artifacts are already on
disk from install; only containers are ever pulled). Images already present locally are
skipped. This is the same step that 'release install' runs by default unless --no-prepare
is passed.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			appToPrepare, err := loadApp(args[0])
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrBadArgument)
			}

			out, _, _ := feedback.OutputStreams()
			err = orchestrator.PrepareRelease(cmd.Context(), appToPrepare, servicelocator.GetDockerClient(), func(message orchestrator.StreamMessage) {
				if message.GetType() == orchestrator.InfoType {
					fmt.Fprintln(out, "[INFO]", message.GetData())
				}
			})
			if err != nil {
				feedback.Fatal(fmt.Sprintf("Failed to prepare images: %s", err), feedback.ErrGeneric)
			}

			feedback.PrintResult(prepareReleaseResult{
				Result:  "ok",
				Message: "Container images prepared",
				AppName: appToPrepare.Name,
			})
			return nil
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveDefault
			}
			return completion.ApplicationNamesWithFilterFunc(cfg, func(apps orchestrator.AppInfo) bool {
				return !apps.Example
			})(cmd, args, toComplete)
		},
	}

	return cmd
}

type prepareReleaseResult struct {
	Result  string `json:"result"`
	Message string `json:"message"`
	AppName string `json:"app_name"`
}

func (r prepareReleaseResult) String() string {
	return fmt.Sprintf("✓ %s for %q", r.Message, r.AppName)
}

func (r prepareReleaseResult) Data() any {
	return r
}
