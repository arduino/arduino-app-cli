package brick

import (
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/results"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

func newBricksListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all available bricks",
		Run: func(cmd *cobra.Command, args []string) {
			bricksListHandler()
		},
	}
}
func bricksListHandler() {
	res, err := orchestrator.BricksList(
		servicelocator.GetModelsIndex(),
		servicelocator.GetBricksIndex(),
	)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	}
	feedback.PrintResult(results.BrickListResult{Bricks: res.Bricks})
}
