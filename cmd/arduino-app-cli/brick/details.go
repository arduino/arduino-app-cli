package brick

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
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
	res, err := orchestrator.BricksDetails(
		servicelocator.GetStaticStore(),
		servicelocator.GetBricksIndex(),
		id,
	)
	if err != nil {
		if errors.Is(err, orchestrator.ErrBrickNotFound) {
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
	BrickDetailsResult orchestrator.BrickDetailsResult
}

func (r brickDetailsResult) String() string {
	b := &strings.Builder{}

	fmt.Fprintf(b, "Name:        %s\n", r.BrickDetailsResult.Name)
	fmt.Fprintf(b, "ID:          %s\n", r.BrickDetailsResult.ID)
	fmt.Fprintf(b, "Author:      %s\n", r.BrickDetailsResult.Author)
	fmt.Fprintf(b, "Category:    %s\n", r.BrickDetailsResult.Category)
	fmt.Fprintf(b, "Status:      %s\n", r.BrickDetailsResult.Status)
	fmt.Fprintf(b, "\nDescription:\n%s\n", r.BrickDetailsResult.Description)

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
