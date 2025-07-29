package properties

import (
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/app"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/results"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

func NewPropertiesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "properties",
		Short: "Manage apps properties",
		Long:  "Manage apps properties, including setting and getting the default app.",
	}

	cmd.AddCommand(&cobra.Command{
		Use:       "get default",
		Short:     "Get properties, e.g., default",
		ValidArgs: []string{"default"},
		Args:      cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			def, err := orchestrator.GetDefaultApp()
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrGeneric)
			}
			feedback.PrintResult(results.DefaultAppResult{
				App: def,
			})
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:       "set default <app_path>",
		Short:     "Set properties, e.g., default",
		Long:      "Set properties. Use 'none' to unset a property.",
		ValidArgs: []string{"default"},
		Args:      cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			// Remove default app.
			if len(args) == 1 || args[1] == "none" {
				if err := orchestrator.SetDefaultApp(nil); err != nil {
					feedback.Fatal(err.Error(), feedback.ErrGeneric)
					return nil
				}
				feedback.PrintResult(results.DefaultAppResult{App: nil})
				return nil
			}

			app, err := app.Load(args[1])
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrBadArgument)
				return nil
			}
			if err := orchestrator.SetDefaultApp(&app); err != nil {
				feedback.Fatal(err.Error(), feedback.ErrGeneric)
				return nil
			}
			feedback.PrintResult(results.DefaultAppResult{App: &app})
			return nil
		},
	})

	return cmd
}
