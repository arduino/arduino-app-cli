// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
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
	appCmd.AddCommand(newRestartCmd(cfg))
	appCmd.AddCommand(newLogsCmd(cfg))
	appCmd.AddCommand(newListCmd(cfg))
	appCmd.AddCommand(newPsCmd())
	appCmd.AddCommand(newMonitorCmd(cfg))

	return appCmd
}

func Load(idOrPath string) (app.ArduinoApp, error) {
	id, err := servicelocator.GetAppIDProvider().ParseID(idOrPath)
	if err != nil {
		return app.ArduinoApp{}, fmt.Errorf("invalid app path: %s", idOrPath)
	}

	return app.Load(id.ToPath().String())
}
