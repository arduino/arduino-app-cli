package main

import (
	"errors"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/cmd/results"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"

	"github.com/spf13/cobra"
)

func newBrickCmd() *cobra.Command {
	appCmd := &cobra.Command{
		Use:   "brick",
		Short: "Manage Arduino Bricks",
	}

	appCmd.AddCommand(newBricksListCmd())
	appCmd.AddCommand(newBricksDetailsCmd())

	return appCmd
}

func newBricksListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all available bricks",
		RunE: func(cmd *cobra.Command, args []string) error {
			return bricksListHandler()
		},
	}
}

func newBricksDetailsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "details",
		Short: "Details of a specific brick",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return bricksDetailsHandler(args[0])
		},
	}
}

func bricksListHandler() error {
	res, err := orchestrator.BricksList(
		servicelocator.GetModelsIndex(),
		servicelocator.GetBricksIndex(),
	)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
		return nil
	}

	feedback.PrintResult(results.BrickListResult{
		Bricks: res.Bricks})
	return nil
}

func bricksDetailsHandler(id string) error {
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
		return nil
	}

	feedback.PrintResult(results.BrickDetailsResult{
		BrickDetailsResult: res,
	})
	return nil
}
