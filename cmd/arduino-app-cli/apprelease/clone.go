// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package apprelease

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newCloneCmd(cfg config.Configuration) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clone release_app new_app_name",
		Short: "Create a new editable app from an installed release",
		Long: `Create a new app from an app that was installed from a release.

The clone is turned back into a live, editable app: the prebuilt sketch binary, the
provisioned Python environment, the release number and the release manifest are removed, so
the next 'app start' will recompile the sketch and re-provision the environment as usual.`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			return cloneHandler(cmd.Context(), cfg, args[0], args[1])
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

func cloneHandler(ctx context.Context, cfg config.Configuration, source string, newName string) error {
	srcApp, err := loadApp(source)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrBadArgument)
	}
	if !srcApp.IsFrozenRelease() {
		feedback.Fatal(fmt.Sprintf("%q is not a release-installed app; use `arduino-app-cli app new --from-app` to clone a regular app", srcApp.Name), feedback.ErrBadArgument)
	}

	id, err := servicelocator.GetAppIDProvider().ParseID(source)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrBadArgument)
	}

	resp, err := orchestrator.CloneApp(ctx, orchestrator.CloneAppRequest{
		FromID:             id,
		Name:               &newName,
		StripFrozenRelease: true,
	}, servicelocator.GetAppIDProvider(), cfg)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	}

	feedback.PrintResult(cloneResult{
		Result:  "ok",
		Message: "App created from release",
		Path:    resp.ID.ToPath().String(),
	})
	return nil
}

type cloneResult struct {
	Result  string `json:"result"`
	Message string `json:"message"`
	Path    string `json:"path"`
}

func (r cloneResult) String() string {
	return fmt.Sprintf("✓ %s at %q", r.Message, r.Path)
}

func (r cloneResult) Data() any {
	return r
}
