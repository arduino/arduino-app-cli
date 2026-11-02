// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package brick

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/remotebricksindex"
)

func newInstallCmd(cfg config.Configuration, version string) *cobra.Command {
	return &cobra.Command{
		Use:     "install ID[@VERSION]",
		Short:   "Install a brick from the bricks index",
		Long:    "Install a brick from the bricks index. Without a version, the latest one is installed.",
		Example: "  arduino-app-cli brick install acme:camera\n  arduino-app-cli brick install acme:camera@1.1.0",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id, brickVersion, err := remotebricksindex.ParseBrickReference(args[0])
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrBadArgument)
			}
			index, err := remotebricksindex.Load(cfg.BricksIndexDir())
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrGeneric)
			}
			brick, release, err := index.FindRelease(id, brickVersion)
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrBadArgument)
			}
			if brick.Deprecated {
				msg := fmt.Sprintf("Brick %s is deprecated", brick.ID)
				if brick.DeprecationNote != "" {
					msg += ": " + brick.DeprecationNote
				}
				feedback.Warnf("%s", msg)
			}

			res, err := remotebricksindex.Install(cmd.Context(), http.DefaultClient, cfg.BricksDir(),
				brick, release, "arduino-app-cli/"+version)
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrGeneric)
			}
			feedback.PrintResult(installResult{
				ID:               brick.ID,
				Version:          release.Version,
				Path:             res.Path.String(),
				AlreadyInstalled: res.AlreadyInstalled,
			})
		},
	}
}

type installResult struct {
	ID               string `json:"id"`
	Version          string `json:"version"`
	Path             string `json:"path"`
	AlreadyInstalled bool   `json:"already_installed"`
}

func (r installResult) String() string {
	if r.AlreadyInstalled {
		return fmt.Sprintf("Brick %s@%s is already installed in %s", r.ID, r.Version, r.Path)
	}
	return fmt.Sprintf("Brick %s@%s installed in %s", r.ID, r.Version, r.Path)
}

func (r installResult) Data() any {
	return r
}
