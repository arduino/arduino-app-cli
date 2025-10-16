package app

import (
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newRestartCmd(cfg config.Configuration) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart app_path",
		Short: "Restart or Start an Arduino App",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			app, err := Load(args[0])
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrBadArgument)
				return nil
			}
			if err := stopHandler(cmd.Context(), app); err != nil {
				feedback.Warnf("failed to stop app: %s", err.Error())
			}
			return startHandler(cmd.Context(), cfg, app)
		},
		ValidArgsFunction: completion.ApplicationNames(cfg),
	}
	return cmd
}
