package main

import (
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/cmd/results"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

func newConfigCmd() *cobra.Command {
	appCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage arduino-app-cli config",
	}

	appCmd.AddCommand(newConfigGetCmd())

	return appCmd
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "get configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return getConfigHandler()
		},
	}
}

func getConfigHandler() error {
	cfg := orchestrator.GetOrchestratorConfig()

	feedback.PrintResult(results.ConfigResult{
		Config: cfg,
	})

	return nil
}
