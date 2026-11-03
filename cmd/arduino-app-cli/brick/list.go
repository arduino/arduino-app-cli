// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package brick

import (
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricks"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/remotebricksindex"
	"github.com/arduino/arduino-app-cli/internal/tablestyle"
)

func newBricksListCmd(cfg config.Configuration) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all available bricks",
		Long:  "List all available bricks: the built-in ones and those installed from the bricks index.",
		Run: func(cmd *cobra.Command, args []string) {
			res := brickListResult{Bricks: []brickListItem{}}
			for _, b := range servicelocator.GetBrickService().List().Bricks {
				res.Bricks = append(res.Bricks, brickListItem{BrickListItem: b})
			}

			installed, err := remotebricksindex.ListInstalled(cfg.BricksDir())
			if err != nil {
				feedback.Warnf("Cannot list the bricks installed from the bricks index: %v", err)
			}
			platform := servicelocator.GetPlatform()
			for _, b := range installed {
				res.Bricks = append(res.Bricks, brickListItem{
					BrickListItem: bricks.BrickListItem{
						ID:           b.Brick.ID,
						Name:         b.Brick.Name,
						Author:       b.Namespace,
						Description:  b.Brick.Description,
						Category:     b.Brick.Category,
						Status:       "installed",
						RequireModel: b.Brick.IsAIBrick(platform),
					},
					Version: b.Version,
				})
			}
			feedback.PrintResult(res)
		},
	}
}

// brickListItem adds the version, which only the bricks installed from the bricks index have.
type brickListItem struct {
	bricks.BrickListItem
	Version string `json:"version,omitempty"`
}

type brickListResult struct {
	Bricks []brickListItem `json:"bricks"`
}

func (r brickListResult) String() string {
	t := table.NewWriter()
	t.SetStyle(tablestyle.CustomCleanStyle)
	t.AppendHeader(table.Row{"ID", "VERSION", "NAME", "AUTHOR"})

	for _, brick := range r.Bricks {
		t.AppendRow(table.Row{
			brick.ID,
			brick.Version,
			brick.Name,
			brick.Author,
		})
	}
	return t.Render()
}

func (r brickListResult) Data() any {
	return r
}
