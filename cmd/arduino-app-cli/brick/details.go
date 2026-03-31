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

package brick

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricks"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newBricksDetailsCmd(cfg config.Configuration) *cobra.Command {
	return &cobra.Command{
		Use:   "details",
		Short: "Details of a specific brick",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			bricksDetailsHandler(args[0], cfg)
		},
		ValidArgsFunction: completion.BrickIDs(),
	}
}

func bricksDetailsHandler(id string, cfg config.Configuration) {
	res, err := servicelocator.GetBrickService().BricksDetails(id, servicelocator.GetAppIDProvider(),
		cfg)
	if err != nil {
		if errors.Is(err, bricks.ErrBrickNotFound) {
			feedback.Fatal(err.Error(), feedback.ErrBadArgument)
		} else {
			feedback.Fatal(err.Error(), feedback.ErrGeneric)
		}
	}

	feedback.PrintResult(brickDetailsResult{
		BrickDetailsResult: res,
	})
}

type brickDetailsResult struct {
	BrickDetailsResult bricks.BrickDetailsResult
}

func (r brickDetailsResult) String() string {
	b := &strings.Builder{}

	b.WriteString("Name:        " + r.BrickDetailsResult.Name + "\n")
	b.WriteString("ID:          " + r.BrickDetailsResult.ID + "\n")
	b.WriteString("Author:      " + r.BrickDetailsResult.Author + "\n")
	b.WriteString("Category:    " + r.BrickDetailsResult.Category + "\n")
	b.WriteString("Status:      " + r.BrickDetailsResult.Status + "\n")
	b.WriteString("\nDescription:\n" + r.BrickDetailsResult.Description + "\n")

	if len(r.BrickDetailsResult.Variables) > 0 {
		b.WriteString("\nVariables:\n")
		for name, variable := range r.BrickDetailsResult.Variables {
			fmt.Fprintf(b, "  - %s (default: '%s', required: %t)\n", name, variable.DefaultValue, variable.Required)
		}
	}

	if r.BrickDetailsResult.Readme != "" {
		b.WriteString("\n--- README ---\n")
		b.WriteString(r.BrickDetailsResult.Readme)
	}

	return b.String()
}

func (r brickDetailsResult) Data() interface{} {
	return r.BrickDetailsResult
}
