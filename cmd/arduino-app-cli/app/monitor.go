package app

import (
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
)

func newMonitorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "monitor",
		Short: "Monitor the Python app",
		RunE: func(cmd *cobra.Command, args []string) error {
			panic("not implemented")
		},
		ValidArgsFunction: completion.ApplicationNames(),
	}
}
