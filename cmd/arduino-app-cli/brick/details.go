package brick

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricks"
)

func newBricksDetailsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "details",
		Short: "Details of a specific brick",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			bricksDetailsHandler(args[0])
		},
	}
}

func bricksDetailsHandler(id string) {
	res, err := servicelocator.GetBrickService().BricksDetails(id)
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
