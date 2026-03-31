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
