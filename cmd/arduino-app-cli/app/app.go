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
