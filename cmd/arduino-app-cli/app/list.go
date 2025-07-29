package app

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/results"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

func newListCmd() *cobra.Command {
	var jsonFormat bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all running Python apps",
		Run: func(cmd *cobra.Command, args []string) {
			listHandler(cmd.Context())
		},
	}

	cmd.Flags().BoolVarP(&jsonFormat, "json", "", false, "Output the list in json format")
	return cmd
}

func listHandler(ctx context.Context) {
	res, err := orchestrator.ListApps(ctx,
		servicelocator.GetDockerClient(),
		orchestrator.ListAppRequest{
			ShowExamples:                   true,
			ShowApps:                       true,
			IncludeNonStandardLocationApps: true,
		},
	)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	}

	feedback.PrintResult(results.AppListResult{
		Apps:       res.Apps,
		BrokenApps: res.BrokenApps,
	})
}
