package app

import (
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newMonitorCmd(cfg config.Configuration) *cobra.Command {
	return &cobra.Command{
		Use:   "monitor",
		Short: "Monitor the Arduino app",
		RunE: func(cmd *cobra.Command, args []string) error {
			panic("not implemented")
		},
		ValidArgsFunction: completion.ApplicationNames(cfg),
	}
}
