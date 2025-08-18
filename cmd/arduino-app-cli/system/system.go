package system

import (
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

func NewSystemCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "system",
		Hidden: true,
	}

	cmd.AddCommand(newDownloadImage())

	return cmd
}

func newDownloadImage() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "init",
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return orchestrator.SystemInit(cmd.Context(), servicelocator.GetUsedPythonImageTag(), servicelocator.GetStaticStore())
		},
	}

	return cmd
}
