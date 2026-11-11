// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package brick

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newUninstallCmd(cfg config.Configuration) *cobra.Command {
	return &cobra.Command{
		Use:     "uninstall ID[@VERSION]...",
		Short:   "Uninstall bricks installed from the bricks index",
		Long:    "Uninstall bricks installed from the bricks index. Without a version, all the installed versions of the brick are uninstalled.",
		Example: "  arduino-app-cli brick uninstall acme:camera\n  arduino-app-cli brick uninstall acme:camera@1.0.0 acme:camera@1.1.0",
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			uninstallHandler(cmd.Context(), args, cfg)
		},
	}
}

func uninstallHandler(ctx context.Context, args []string, cfg config.Configuration) {
	var res uninstallResult
	for _, ref := range parseBrickReferences(args) {
		uninstalled, err := bricksindex.Uninstall(ctx, cfg.BricksDir(), ref.id, ref.version)
		if err == nil || len(uninstalled.Versions) > 0 {
			versions := uninstalled.Versions
			if versions == nil {
				versions = []string{}
			}
			res.Bricks = append(res.Bricks, uninstalledBrick{
				ID:               ref.id,
				RequestedVersion: ref.version,
				Versions:         versions,
			})
		}
		if err != nil {
			if len(res.Bricks) > 0 {
				feedback.PrintResult(res)
			}
			feedback.Fatal(err.Error(), feedback.ErrGeneric)
		}
	}
	feedback.PrintResult(res)
}

type uninstalledBrick struct {
	ID               string   `json:"id"`
	RequestedVersion string   `json:"-"`
	Versions         []string `json:"versions"`
}

type uninstallResult struct {
	Bricks []uninstalledBrick `json:"bricks"`
}

func (r uninstallResult) String() string {
	lines := make([]string, 0, len(r.Bricks))
	for _, b := range r.Bricks {
		switch {
		case len(b.Versions) > 0:
			lines = append(lines, fmt.Sprintf("Brick %s uninstalled: %s", b.ID, strings.Join(b.Versions, ", ")))
		case b.RequestedVersion != "":
			lines = append(lines, fmt.Sprintf("Brick %s@%s is not installed", b.ID, b.RequestedVersion))
		default:
			lines = append(lines, fmt.Sprintf("Brick %s is not installed", b.ID))
		}
	}
	return strings.Join(lines, "\n")
}

func (r uninstallResult) Data() any {
	return r
}
