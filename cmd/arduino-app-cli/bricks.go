package main

import (
	"encoding/json"
	"fmt"

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
	res, err := orchestrator.BricksList()
	if err != nil {
		return nil
	}

	resJSON, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return nil
	}
	fmt.Println(string(resJSON))
	return nil
}

func bricksDetailsHandler(id string) error {
	res, err := orchestrator.BricksDetails(id)
	if err != nil {
		return nil
	}

	resJSON, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return nil
	}
	fmt.Println(string(resJSON))
	return nil
}
