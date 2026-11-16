// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package brick

import (
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func NewBrickCmd(cfg config.Configuration, version string) *cobra.Command {
	appCmd := &cobra.Command{
		Use:   "brick",
		Short: "Manage Arduino Bricks",
	}

	appCmd.AddCommand(newBricksListCmd())
	appCmd.AddCommand(newBricksDetailsCmd(cfg))
	appCmd.AddCommand(newUpdateIndexCmd(cfg, version))
	appCmd.AddCommand(newSearchCmd(cfg, version))
	appCmd.AddCommand(newInstallCmd(cfg, version))
	appCmd.AddCommand(newUninstallCmd(cfg))

	return appCmd
}
