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
	var showBrokenApps bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the Arduino apps",
		Run: func(cmd *cobra.Command, args []string) {
			listHandler(cmd.Context(), cfg, showBrokenApps)
		},
	}

	cmd.Flags().BoolVarP(&showBrokenApps, "show-broken-apps", "", false, "Output a list of broken apps")
	return cmd
}

func listHandler(ctx context.Context, cfg config.Configuration, showBrokenApps bool) {
	res, err := orchestrator.ListApps(ctx,
		servicelocator.GetDockerClient(),
		orchestrator.ListAppRequest{
			ShowExamples:                   true,
			ShowApps:                       true,
			IncludeNonStandardLocationApps: true,
		},
		servicelocator.GetAppIDProvider(),
		cfg,
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
	t.AppendHeader(table.Row{"ID", "NAME", "ICON", "STATUS", "EXAMPLE"})

	for _, app := range r.Apps {
		t.AppendRow(table.Row{
			cmdutil.IDToAlias(app.ID),
			app.Name,
			app.Icon,
			app.Status,
			app.Example,
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

func (r appListResult) Data() interface{} {
	return r
}
