package brick

import (
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func NewBrickCmd(cfg config.Configuration) *cobra.Command {
	appCmd := &cobra.Command{
		Use:   "brick",
		Short: "Manage Arduino Bricks",
	}

	appCmd.AddCommand(newBricksListCmd())
	appCmd.AddCommand(newBricksDetailsCmd(cfg))

	return appCmd
}
