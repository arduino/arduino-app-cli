package brick

import (
	"github.com/spf13/cobra"
)

func NewBrickCmd() *cobra.Command {
	appCmd := &cobra.Command{
		Use:   "brick",
		Short: "Manage Arduino Bricks",
	}

	appCmd.AddCommand(newBricksListCmd())
	appCmd.AddCommand(newBricksDetailsCmd())

	return appCmd
}
