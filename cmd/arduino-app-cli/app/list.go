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
	var showReleases bool
	var showAll bool
	var showBrokenApps bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the Arduino apps catalog",
		Long: "List the Arduino apps catalog.\n" +
			"By default the user apps and the apps installed from a release are shown. " +
			"Use --examples to list the example apps, --releases to list only the apps " +
			"installed from a release, or --all to list everything.\n" +
			"To see the live status of the apps on the board, use 'app ps' instead.",
		Run: func(cmd *cobra.Command, args []string) {
			listHandler(cmd.Context(), cfg, showExamples, showReleases, showAll, showBrokenApps)
		},
	}

	cmd.Flags().BoolVar(&showExamples, "examples", false, "Only list the example apps")
	cmd.Flags().BoolVar(&showReleases, "releases", false, "Only list the apps installed from a release")
	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "List user apps, example apps and apps installed from a release")
	cmd.Flags().BoolVarP(&showBrokenApps, "show-broken-apps", "", false, "Output a list of broken apps")
	return cmd
}

func listHandler(ctx context.Context, cfg config.Configuration, showExamples, showReleases, showAll, showBrokenApps bool) {
	// By default we show the user apps and the ones installed from a release. --examples
	// or --releases restrict the view to one of them, and --all shows everything.
	restricted := showExamples || showReleases

	res, err := orchestrator.ListApps(ctx,
		servicelocator.GetDockerClient(),
		orchestrator.ListAppRequest{
			ShowApps:     showAll || !restricted,
			ShowExamples: showAll || showExamples,
			ShowReleases: showAll || showReleases || !restricted,
		},
		servicelocator.GetAppIDProvider(),
		servicelocator.GetBricksIndex(),
		cfg,
		servicelocator.GetPlatform(),
	)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	}

	// When calling the "app list" without flags, notify the users of a breaking behavior change.
	if !restricted && !showAll && feedback.GetFormat() == feedback.Text {
		feedback.Warnf("Note: 'app list' is now a catalog view. The STATUS column has been removed: " +
			"use 'app ps' to see the apps running on the board. Example apps are no longer listed by " +
			"default: use --examples to list them, or --all to list apps and examples together.")
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
	t.AppendHeader(table.Row{"ID", "NAME", "ICON"})

	for _, app := range r.Apps {
		name := app.Name
		if app.Default {
			name += " *"
		}
		t.AppendRow(table.Row{
			cmdutil.IDToAlias(app.ID),
			name,
			app.Icon,
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
