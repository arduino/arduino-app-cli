package version

import (
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/results"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
)

func NewVersionCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version number of Arduino App CLI",
		Run: func(cmd *cobra.Command, args []string) {
			feedback.PrintResult(results.VersionResult{
				AppName: "Arduino App CLI",
				Version: version,
			})
		},
	}
	return cmd
}
