// This file is part of arduino-app-cli.
//
// Copyright Copyright (C) Arduino s.r.l. and/or its affiliated companies
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
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newCacheCleanCmd(cfg config.Configuration) *cobra.Command {
	var forceClean bool
	appCmd := &cobra.Command{
		Use:   "clean-cache <app-id>",
		Short: "Delete app cache",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := Load(args[0])
			if err != nil {
				return err
			}
			return cacheCleanHandler(cmd.Context(), app, forceClean)
		},
		ValidArgsFunction: completion.ApplicationNames(cfg),
	}
	appCmd.Flags().BoolVarP(&forceClean, "force", "", false, "Forcefully clean the cache even if the app is running")

	return appCmd
}

func cacheCleanHandler(ctx context.Context, app app.ArduinoApp, forceClean bool) error {
	err := orchestrator.CleanAppCache(
		ctx,
		servicelocator.GetDockerClient(),
		app,
		orchestrator.CleanAppCacheRequest{ForceClean: forceClean},
		servicelocator.GetPlatform(),
	)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	}
	feedback.PrintResult(cacheCleanResult{
		AppName: app.Name,
		Path:    app.ProvisioningStateDir().String(),
	})
	return nil
}

type cacheCleanResult struct {
	AppName string `json:"appName"`
	Path    string `json:"path"`
}

func (r cacheCleanResult) String() string {
	return fmt.Sprintf("✓ Cache of %q App cleaned", r.AppName)
}

func (r cacheCleanResult) Data() interface{} {
	return r
}
