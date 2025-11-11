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

	b.WriteString(fmt.Sprintf("Data Directory:     %s\n", r.Config.Directories.Data))
	b.WriteString(fmt.Sprintf("Apps Directory:     %s\n", r.Config.Directories.Apps))
	b.WriteString(fmt.Sprintf("Examples Directory: %s\n", r.Config.Directories.Examples))

	return b.String()
}

func (r configResult) Data() interface{} {
	return r.Config
}
