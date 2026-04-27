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

package completion

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/cmdutil"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricks"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func NewCompletionCommand() *cobra.Command {
	completionCmd := &cobra.Command{
		Use:       "completion [bash|zsh|fish|powershell]",
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Args:      cobra.ExactArgs(1),
		Short:     "Generates completion scripts",
		Long:      "Generates completion scripts for various shells",
		Example: "  " + os.Args[0] + " completion bash > completion.sh\n" +
			"  " + "source completion.sh",
		RunE: func(cmd *cobra.Command, args []string) error {
			stdout, _, err := feedback.DirectStreams()
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrBadArgument)
				return nil
			}
			completionNoDesc, _ := cmd.Flags().GetBool("no-descriptions")

			shell := args[0]
			switch shell {
			case "bash":
				return cmd.Root().GenBashCompletionV2(stdout, !completionNoDesc)
			case "zsh":
				if completionNoDesc {
					return cmd.Root().GenZshCompletionNoDesc(stdout)
				}
				return cmd.Root().GenZshCompletion(stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(stdout, !completionNoDesc)
			case "powershell":
				return cmd.Root().GenPowerShellCompletion(stdout)
			default:
				return cmd.Usage() // Handle invalid shell argument
			}
		},
	}

	completionCmd.Flags().Bool("no-descriptions", false, "Disable completion description for shells that support it")

	return completionCmd
}

func ApplicationNames(cfg config.Configuration) cobra.CompletionFunc {
	return ApplicationNamesWithFilterFunc(cfg, func(_ orchestrator.AppInfo) bool { return true })
}

func ApplicationNamesWithFilterFunc(cfg config.Configuration, filter func(apps orchestrator.AppInfo) bool) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		apps, err := orchestrator.ListApps(cmd.Context(),
			servicelocator.GetDockerClient(),
			orchestrator.ListAppRequest{
				ShowExamples:                   true,
				ShowApps:                       true,
				IncludeNonStandardLocationApps: true,
			},
			servicelocator.GetAppIDProvider(),
			servicelocator.GetBricksIndex(),
			cfg,
		)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		var res []string
		for _, a := range apps.Apps {
			if filter(a) {
				res = append(res, cmdutil.IDToAlias(a.ID))
			}
		}
		return res, cobra.ShellCompDirectiveDefault
	}
}

func BrickIDs() cobra.CompletionFunc {
	return BrickIDsWithFilterFunc(func(_ bricks.BrickListItem) bool { return true })
}

func BrickIDsWithFilterFunc(filter func(apps bricks.BrickListItem) bool) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		brickList, err := servicelocator.GetBrickService().List()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		var res []string
		for _, brick := range brickList.Bricks {
			if filter(brick) {
				res = append(res, brick.ID)
			}
		}
		return res, cobra.ShellCompDirectiveNoFileComp
	}
}
