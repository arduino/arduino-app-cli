// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func NewAppCmd(cfg config.Configuration) *cobra.Command {
	appCmd := &cobra.Command{
		Use:   "app",
		Short: "Manage Arduino Apps",
		Long:  "A CLI tool to manage Arduino Apps, including starting, stopping, logging, and provisioning.",
	}

	appCmd.AddCommand(newCreateCmd(cfg))
	appCmd.AddCommand(newStartCmd(cfg))
	appCmd.AddCommand(newStopCmd(cfg))
	appCmd.AddCommand(newDestroyCmd(cfg))
	appCmd.AddCommand(newRestartCmd(cfg))
	appCmd.AddCommand(newLogsCmd(cfg))
	appCmd.AddCommand(newListCmd(cfg))
	appCmd.AddCommand(newCacheCleanCmd(cfg))
	appCmd.AddCommand(newExportCmd(cfg))
	appCmd.AddCommand(newImportCmd(cfg))

	return appCmd
}

func Load(idOrPath string) (app.ArduinoApp, error) {
	id, err := servicelocator.GetAppIDProvider().ParseID(idOrPath)
	if err != nil {
		return app.ArduinoApp{}, fmt.Errorf("invalid app path: %s", idOrPath)
	}

	return app.Load(id.ToPath())
}
