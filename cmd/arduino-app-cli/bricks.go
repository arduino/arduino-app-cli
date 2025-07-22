package main

import (
	"encoding/json"
	"fmt"
	"io/fs"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"

	"github.com/spf13/cobra"
)

func newBrickCmd(
	brickDocsFS fs.FS,
	modelsIndex *modelsindex.ModelsIndex,
	bricksIndex *bricksindex.BricksIndex,
) *cobra.Command {
	appCmd := &cobra.Command{
		Use:   "brick",
		Short: "Manage Arduino Bricks",
	}

	appCmd.AddCommand(newBricksListCmd(modelsIndex, bricksIndex))
	appCmd.AddCommand(newBricksDetailsCmd(brickDocsFS, bricksIndex))

	return appCmd
}

func newBricksListCmd(
	modelsIndex *modelsindex.ModelsIndex,
	bricksIndex *bricksindex.BricksIndex,
) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all available bricks",
		RunE: func(cmd *cobra.Command, args []string) error {
			return bricksListHandler(modelsIndex, bricksIndex)
		},
	}
}

func newBricksDetailsCmd(
	brickDocsFS fs.FS,
	bricksIndex *bricksindex.BricksIndex,
) *cobra.Command {
	return &cobra.Command{
		Use:   "details",
		Short: "Details of a specific brick",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return bricksDetailsHandler(brickDocsFS, bricksIndex, args[0])
		},
	}
}

func bricksListHandler(
	modelsIndex *modelsindex.ModelsIndex,
	bricksIndex *bricksindex.BricksIndex,
) error {
	res, err := orchestrator.BricksList(modelsIndex, bricksIndex)
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

func bricksDetailsHandler(
	brickDocsFS fs.FS,
	bricksIndex *bricksindex.BricksIndex,
	id string,
) error {
	res, err := orchestrator.BricksDetails(brickDocsFS, bricksIndex, id)
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
