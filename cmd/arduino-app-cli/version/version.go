package version

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
)

func NewVersionCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version number of Arduino App CLI",
		Run: func(cmd *cobra.Command, args []string) {
			feedback.PrintResult(versionResult{
				AppName: "Arduino App CLI",
				Version: version,
			})
		},
	}
	return cmd
}

type versionResult struct {
	AppName string `json:"appName"`
	Version string `json:"version"`
}

func (r versionResult) String() string {
	return fmt.Sprintf("%s v%s", r.AppName, r.Version)
}

func (r versionResult) Data() interface{} {
	return r
}
