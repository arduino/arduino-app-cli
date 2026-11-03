// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package brick

import (
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/remotebricksindex"
	"github.com/arduino/arduino-app-cli/internal/tablestyle"
)

func newSearchCmd(cfg config.Configuration) *cobra.Command {
	var allVersions bool
	cmd := &cobra.Command{
		Use:     "search [keywords...]",
		Short:   "Search bricks in the bricks index",
		Long:    "Search bricks in the bricks index. Without keywords, all the bricks are listed.",
		Example: "  arduino-app-cli brick search camera usb --all",
		Args:    cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			searchHandler(args, allVersions, cfg)
		},
	}
	cmd.Flags().BoolVarP(&allVersions, "all", "a", false, "Show all the versions of the bricks")
	return cmd
}

func searchHandler(keywords []string, allVersions bool, cfg config.Configuration) {
	index, err := remotebricksindex.Load(cfg.BricksIndexDir())
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	}
	feedback.PrintResult(searchResult{
		Bricks:      index.Search(strings.Join(keywords, " ")),
		allVersions: allVersions,
	})
}

type searchResult struct {
	Bricks      []remotebricksindex.Brick `json:"bricks"`
	allVersions bool
}

func (r searchResult) String() string {
	if len(r.Bricks) == 0 {
		return "No bricks matching your search."
	}

	t := table.NewWriter()
	t.SetStyle(tablestyle.CustomCleanStyle)
	t.AppendHeader(table.Row{"ID", "VERSION", "NAME", "DEPRECATED"})
	for _, b := range r.Bricks {
		deprecated := ""
		if b.Deprecated {
			deprecated = "yes"
		}
		if !r.allVersions {
			t.AppendRow(table.Row{b.ID, b.Latest, b.Name, deprecated})
			continue
		}
		for _, release := range b.Releases {
			t.AppendRow(table.Row{b.ID, release.Version, b.Name, deprecated})
		}
	}
	return t.Render()
}

func (r searchResult) Data() any {
	return r
}
