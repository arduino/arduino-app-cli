// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"context"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/cmdutil"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/tablestyle"
)

// psVisibleStatuses are the statuses shown by default, i.e. any app that has
// been initialized at least once and is not currently stopped.
var psVisibleStatuses = map[orchestrator.Status]bool{
	orchestrator.StatusStarting: true,
	orchestrator.StatusRunning:  true,
	orchestrator.StatusStopping: true,
	orchestrator.StatusFailed:   true,
}

func newPsCmd() *cobra.Command {
	var showAll bool

	cmd := &cobra.Command{
		Use:   "ps",
		Short: "List the status of the Arduino apps on the board",
		Long: "List the Arduino apps that have been initialized at least once, and their live status.\n" +
			"By default, only starting, running, stopping and failed apps are shown. Use --all to also " +
			"include stopped apps. Apps that have never been started are never shown; use 'app list' to " +
			"browse the full catalog instead.",
		Run: func(cmd *cobra.Command, args []string) {
			psHandler(cmd.Context(), showAll)
		},
	}

	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "Also show stopped apps")
	return cmd
}

func psHandler(ctx context.Context, showAll bool) {
	apps, err := orchestrator.ListActiveApps(
		ctx,
		servicelocator.GetDockerClient(),
		servicelocator.GetAppIDProvider(),
	)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	}

	feedback.PrintResult(appPsResult{
		Apps: filterAppsByStatus(apps, showAll),
	})
}

// filterAppsByStatus keeps only apps that have been initialized at least once
// (i.e. excludes StatusUninitialized), showing stopped apps only if showAll is set.
func filterAppsByStatus(apps []orchestrator.AppInfo, showAll bool) []orchestrator.AppInfo {
	res := make([]orchestrator.AppInfo, 0, len(apps))
	for _, a := range apps {
      switch a.Status {
              case orchestrator.StatusStarting, orchestrator.StatusRunning, orchestrator.StatusStopping, orchestrator.StatusFailed:
                      res = append(res, a)
              case orchestrator.StatusStopped:
                      if showAll {
                              res = append(res, a)
                      }
              }
	}
	return res
}

type appPsResult struct {
	Apps []orchestrator.AppInfo `json:"apps"`
}

func (r appPsResult) String() string {
	t := table.NewWriter()
	t.SetStyle(tablestyle.CustomCleanStyle)
	t.AppendHeader(table.Row{"ID", "STATUS", "NAME"})

	for _, app := range r.Apps {
		t.AppendRow(table.Row{
			cmdutil.IDToAlias(app.ID),
			app.Name,
			app.Status,
		})
	}
	return t.Render()
}

func (r appPsResult) Data() any {
	return r
}
