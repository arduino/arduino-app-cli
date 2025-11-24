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
