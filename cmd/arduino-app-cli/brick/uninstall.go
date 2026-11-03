// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package brick

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/remotebricksindex"
)

func newUninstallCmd(cfg config.Configuration) *cobra.Command {
	return &cobra.Command{
		Use:     "uninstall ID[@VERSION]",
		Short:   "Uninstall a brick installed from the bricks index",
		Long:    "Uninstall a brick installed from the bricks index. Without a version, all the installed versions are uninstalled.",
		Example: "  arduino-app-cli brick uninstall acme:camera\n  arduino-app-cli brick uninstall acme:camera@1.1.0",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id, brickVersion, err := remotebricksindex.ParseBrickReference(args[0])
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrBadArgument)
			}
			res, err := remotebricksindex.Uninstall(cmd.Context(), cfg.BricksDir(), id, brickVersion)
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrGeneric)
			}
			feedback.PrintResult(uninstallResult{
				ID:               id,
				RequestedVersion: brickVersion,
				Versions:         res.Versions,
			})
		},
	}
}

type uninstallResult struct {
	ID               string   `json:"id"`
	RequestedVersion string   `json:"-"`
	Versions         []string `json:"versions"`
}

func (r uninstallResult) String() string {
	if len(r.Versions) == 0 {
		if r.RequestedVersion != "" {
			return fmt.Sprintf("Brick %s@%s is not installed", r.ID, r.RequestedVersion)
		}
		return fmt.Sprintf("Brick %s is not installed", r.ID)
	}
	return fmt.Sprintf("Brick %s uninstalled: %s", r.ID, strings.Join(r.Versions, ", "))
}

func (r uninstallResult) Data() any {
	if r.Versions == nil {
		r.Versions = []string{}
	}
	return r
}
