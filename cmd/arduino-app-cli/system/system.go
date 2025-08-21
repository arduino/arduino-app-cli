package system

import (
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func NewSystemCmd(cfg config.Configuration) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "system",
		Hidden: true,
	}

	cmd.AddCommand(newDownloadImage(cfg))

	return cmd
}

func newDownloadImage(cfg config.Configuration) *cobra.Command {
	cmd := &cobra.Command{
		Use:  "init",
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return orchestrator.SystemInit(cmd.Context(), cfg.UsedPythonImageTag, servicelocator.GetStaticStore())
		},
	}

	return cmd
}
