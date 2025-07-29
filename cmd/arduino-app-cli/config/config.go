package config

import (
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/results"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

func NewConfigCmd() *cobra.Command {
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
		Run: func(cmd *cobra.Command, args []string) {
			getConfigHandler()
		},
	}
}

func getConfigHandler() {
	feedback.PrintResult(results.ConfigResult{
		Config: orchestrator.GetOrchestratorConfig(),
	})
}
