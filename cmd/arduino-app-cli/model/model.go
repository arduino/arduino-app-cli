// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package model

import (
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func NewModelCmd(cfg config.Configuration) *cobra.Command {
	modelCmd := &cobra.Command{
		Use:   "model",
		Short: "Manage Arduino Models",
	}

	modelCmd.AddCommand(newModelListCmd())
	modelCmd.AddCommand(newModelDeleteCmd(cfg))

	return modelCmd
}
