package config

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

func NewConfigCmd() *cobra.Command {
	appCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage arduino-app-cli config",
	}

	appCmd.AddCommand(newConfigGetCmd())

	return appCmd
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "get configuration",
		Run: func(cmd *cobra.Command, args []string) {
			getConfigHandler()
		},
	}
}

func getConfigHandler() {
	feedback.PrintResult(configResult{
		Config: orchestrator.GetOrchestratorConfig(),
	})
}

type configResult struct {
	Config orchestrator.ConfigResponse
}

func (r configResult) String() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Data Directory:     %s\n", r.Config.Directories.Data))
	b.WriteString(fmt.Sprintf("Apps Directory:     %s\n", r.Config.Directories.Apps))
	b.WriteString(fmt.Sprintf("Examples Directory: %s\n", r.Config.Directories.Examples))

	return b.String()
}

func (r configResult) Data() interface{} {
	return r.Config
}
