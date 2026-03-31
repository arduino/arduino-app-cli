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

package config

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func NewConfigCmd(cfg config.Configuration) *cobra.Command {
	appCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Arduino App CLI config",
	}

	appCmd.AddCommand(newConfigGetCmd(cfg))

	return appCmd
}

func newConfigGetCmd(cfg config.Configuration) *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "get configuration",
		Run: func(cmd *cobra.Command, args []string) {
			getConfigHandler(cfg)
		},
	}
}

func getConfigHandler(cfg config.Configuration) {
	feedback.PrintResult(configResult{
		Config: orchestrator.GetOrchestratorConfig(cfg),
	})
}

type configResult struct {
	Config orchestrator.ConfigResponse
}

func (r configResult) String() string {
	var b strings.Builder

	fmt.Fprintf(&b, "Data Directory:     %s\n", r.Config.Directories.Data)
	fmt.Fprintf(&b, "Apps Directory:     %s\n", r.Config.Directories.Apps)
	fmt.Fprintf(&b, "Examples Directory: %s\n", r.Config.Directories.Examples)

	return b.String()
}

func (r configResult) Data() interface{} {
	return r.Config
}
