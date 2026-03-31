// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package properties

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/app"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	arduinoApp "github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func NewPropertiesCmd(cfg config.Configuration) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "properties",
		Short: "Manage apps properties",
		Long:  "Manage apps properties, including setting and getting the default app.",
	}

	cmd.AddCommand(&cobra.Command{
		Use:       "get default",
		Short:     "Get properties, e.g., default",
		ValidArgs: []string{"default"},
		Args:      cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			def, err := orchestrator.GetDefaultApp(cfg)
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrGeneric)
			}
			feedback.PrintResult(defaultAppResult{App: def})
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:       "set default <app_path>",
		Short:     "Set properties, e.g., default",
		Long:      "Set properties. Use 'none' to unset a property.",
		ValidArgs: []string{"default"},
		Args:      cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			// Remove default app.
			if len(args) == 1 || args[1] == "none" {
				if err := orchestrator.SetDefaultApp(nil, cfg); err != nil {
					feedback.Fatal(err.Error(), feedback.ErrGeneric)
					return nil
				}
				feedback.PrintResult(defaultAppResult{App: nil})
				return nil
			}

			app, err := app.Load(args[1])
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrBadArgument)
				return nil
			}
			if err := orchestrator.SetDefaultApp(&app, cfg); err != nil {
				feedback.Fatal(err.Error(), feedback.ErrGeneric)
				return nil
			}
			feedback.PrintResult(defaultAppResult{App: &app})
			return nil
		},
	})

	return cmd
}

type defaultAppResult struct {
	App *arduinoApp.ArduinoApp `json:"app,omitempty"`
}

func (r defaultAppResult) String() string {
	if r.App == nil {
		return "No default app set"
	}
	return fmt.Sprintf("Default app: %s (%s)", r.App.Name, r.App.FullPath)
}

func (r defaultAppResult) Data() interface{} {
	return r
}
