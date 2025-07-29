package brick

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/results"
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
		servicelocator.GetBricksDocsFS(),
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

	feedback.PrintResult(results.BrickDetailsResult{
		BrickDetailsResult: res,
	})
}
