package app

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

func NewAppCmd() *cobra.Command {
	appCmd := &cobra.Command{
		Use:   "app",
		Short: "Manage Arduino Apps",
		Long:  "A CLI tool to manage Arduino Apps, including starting, stopping, logging, and provisioning.",
	}

	appCmd.AddCommand(newCreateCmd())
	appCmd.AddCommand(newStartCmd())
	appCmd.AddCommand(newStopCmd())
	appCmd.AddCommand(newRestartCmd())
	appCmd.AddCommand(newLogsCmd())
	appCmd.AddCommand(newListCmd())
	appCmd.AddCommand(newPsCmd())
	appCmd.AddCommand(newMonitorCmd())

	return appCmd
}

func Load(idOrPath string) (app.ArduinoApp, error) {
	id, err := orchestrator.ParseID(idOrPath)
	if err != nil {
		return app.ArduinoApp{}, fmt.Errorf("invalid app path: %s", idOrPath)
	}

	return app.Load(id.ToPath().String())
}
