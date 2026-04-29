// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

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
