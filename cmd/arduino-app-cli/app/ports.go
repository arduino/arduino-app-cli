// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/tablestyle"
)

func newPortsCmd(cfg config.Configuration) *cobra.Command {
	return &cobra.Command{
		Use:   "ports app_path",
		Short: "Show the ports exposed by an app",
		Long: "Show the ports exposed by an app: the ones declared by its app.yaml, the ones declared by its " +
			"bricks, and the ones published by the services the bricks require.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			app, err := Load(args[0])
			if err != nil {
				return err
			}
			return portsHandler(app)
		},
		ValidArgsFunction: completion.ApplicationNames(cfg),
	}
}

func portsHandler(app app.ArduinoApp) error {
	ports, err := orchestrator.AppPorts(app, servicelocator.GetBricksIndex(), servicelocator.GetServicesIndex())
	if err != nil {
		return err
	}

	feedback.PrintResult(appPortsResult{Ports: ports})
	return nil
}

type appPortsResult struct {
	Ports []orchestrator.PortInfo `json:"ports"`
}

func (r appPortsResult) String() string {
	if len(r.Ports) == 0 {
		return "The app does not expose any port."
	}

	t := table.NewWriter()
	t.SetStyle(tablestyle.CustomCleanStyle)
	t.AppendHeader(table.Row{"PORT", "SOURCE", "TYPE", "INTENT"})

	for _, port := range r.Ports {
		t.AppendRow(table.Row{port.Port, port.Source, port.SourceType, port.Intent})
	}

	return t.Render()
}

func (r appPortsResult) Data() any {
	return r
}
