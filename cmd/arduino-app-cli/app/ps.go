package app

import (
	"github.com/spf13/cobra"
)

func newPsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ps",
		Short: "Shows the list of running Arduino Apps",
		RunE: func(cmd *cobra.Command, args []string) error {
			panic("not implemented")
		},
	}
}
