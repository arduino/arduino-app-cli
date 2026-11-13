// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package brick

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
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

// failIfRoot exits when running as root: the files it would create could not be modified by the
// arduino user.
func failIfRoot(action string) {
	if os.Geteuid() == 0 {
		feedback.Fatal(fmt.Sprintf("cannot %s as root. Try `su - arduino` before this command.", action), feedback.ErrGeneric)
	}
}

// printWaiting tells the user that the command is waiting for another brick operation to finish.
func printWaiting() {
	out, _, _ := feedback.OutputStreams()
	fmt.Fprintln(out, "Waiting for another brick operation to finish...")
}
