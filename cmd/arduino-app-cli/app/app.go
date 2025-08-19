package app

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func NewAppCmd(cfg config.Configuration) *cobra.Command {
	appCmd := &cobra.Command{
		Use:   "app",
		Short: "Manage Arduino Apps",
		Long:  "A CLI tool to manage Arduino Apps, including starting, stopping, logging, and provisioning.",
	}

	appCmd.AddCommand(newCreateCmd(cfg))
	appCmd.AddCommand(newStartCmd(cfg))
	appCmd.AddCommand(newStopCmd(cfg))
	appCmd.AddCommand(newRestartCmd(cfg))
	appCmd.AddCommand(newLogsCmd(cfg))
	appCmd.AddCommand(newListCmd(cfg))
	appCmd.AddCommand(newPsCmd())
	appCmd.AddCommand(newMonitorCmd(cfg))

	return appCmd
}

func Load(idOrPath string) (app.ArduinoApp, error) {
	id, err := servicelocator.GetAppIDProvider().ParseID(idOrPath)
	if err != nil {
		return app.ArduinoApp{}, fmt.Errorf("invalid app path: %s", idOrPath)
	}

	return app.Load(id.ToPath().String())
}
