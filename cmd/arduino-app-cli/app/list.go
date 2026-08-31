// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"context"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/cmdutil"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/tablestyle"
)

func newListCmd(cfg config.Configuration) *cobra.Command {
	var showExamples bool
	var showAll bool
	var showBrokenApps bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the Arduino apps catalog",
		Long: "List the Arduino apps catalog.\n" +
			"By default only user apps are shown. Use --examples to list the example apps, " +
			"or --all to list both.\n" +
			"To see the live status of the apps on the board, use 'app ps' instead.",
		Run: func(cmd *cobra.Command, args []string) {
			listHandler(cmd.Context(), cfg, showExamples, showAll, showBrokenApps)
		},
	}

	cmd.Flags().BoolVar(&showExamples, "examples", false, "Only list the example apps")
	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "List both user apps and example apps")
	cmd.Flags().BoolVarP(&showBrokenApps, "show-broken-apps", "", false, "Output a list of broken apps")
	return cmd
}

func listHandler(ctx context.Context, cfg config.Configuration, showExamples, showAll, showBrokenApps bool) {
	res, err := orchestrator.ListApps(ctx,
		servicelocator.GetDockerClient(),
		orchestrator.ListAppRequest{
			// By default we only show user apps. --examples restricts the view to examples only,
			// and --all shows both.
			ShowApps:                       showAll || !showExamples,
			ShowExamples:                   showAll || showExamples,
			IncludeNonStandardLocationApps: true,
		},
		servicelocator.GetAppIDProvider(),
		servicelocator.GetBricksIndex(),
		cfg,
		servicelocator.GetPlatform(),
	)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	}

	feedback.PrintResult(appListResult{
		Apps:           res.Apps,
		BrokenApps:     res.BrokenApps,
		showBrokenApps: showBrokenApps,
	})
}

type appListResult struct {
	Apps           []orchestrator.AppInfo       `json:"apps"`
	BrokenApps     []orchestrator.BrokenAppInfo `json:"brokenApps"`
	showBrokenApps bool
}

func (r appListResult) String() string {
	t := table.NewWriter()
	t.SetStyle(tablestyle.CustomCleanStyle)
	t.AppendHeader(table.Row{"ID", "NAME", "DESCRIPTION"})

	for _, app := range r.Apps {
		t.AppendRow(table.Row{
			cmdutil.IDToAlias(app.ID),
			app.Name,
			app.Description,
		})
	}
	if r.showBrokenApps && len(r.BrokenApps) > 0 {
		var b strings.Builder
		_, _ = b.WriteString("\nBROKEN APPS\n")
		for _, app := range r.BrokenApps {
			b.WriteString(app.Name + ": " + app.Error + "\n")
		}
		return t.Render() + "\n" + b.String()
	}
	return t.Render()
}

func (r appListResult) Data() any {
	return r
}
