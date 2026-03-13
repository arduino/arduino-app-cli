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

package model

import (
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/tablestyle"
)

func newModelListCmd() *cobra.Command {
	var excludeBuiltin bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all models",
		Run: func(cmd *cobra.Command, args []string) {
			modelListHandler(excludeBuiltin)
		},
	}

	cmd.Flags().BoolVar(&excludeBuiltin, "exclude-builtin", false, "Do not show Arduino builtin models.")

	return cmd
}

func modelListHandler(excludeBuiltin bool) {
	models := servicelocator.GetModelsIndex().GetModels()
	result := make([]modelsindex.AIModel, 0)
	for _, m := range models {
		if excludeBuiltin && m.IsInternal {
			continue
		}
		result = append(result, m)
	}
	feedback.PrintResult(modelListResult{Models: result})
}

type modelListResult struct {
	Models []modelsindex.AIModel `json:"models"`
}

func (r modelListResult) String() string {
	t := table.NewWriter()
	t.SetStyle(tablestyle.CustomCleanStyle)
	t.AppendHeader(table.Row{"ID", "NAME", "BUILTIN"})

	for _, model := range r.Models {
		checkmark := ""
		if model.IsInternal {
			checkmark = "✓"
		}
		t.AppendRow(table.Row{
			model.ID,
			model.Name,
			checkmark,
		})
	}
	return t.Render()
}

func (r modelListResult) Data() any {
	return r
}
