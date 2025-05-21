package main

import (
	"context"
	"fmt"
	"log/slog"

	dockerClient "github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"go.bug.st/cleanup"
)

// Version will be set a build time with -ldflags
var Version string = "0.0.0-dev"

func main() {
	docker, err := dockerClient.NewClientWithOpts(
		dockerClient.FromEnv,
		dockerClient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		panic(err)
	}
	defer docker.Close()

	rootCmd := &cobra.Command{
		Use:   "arduino-app-cli",
		Short: "A CLI to manage the Python app",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.AddCommand(
		newAppCmd(docker),
		newCompletionCommand(),
		newDaemonCmd(docker),
		&cobra.Command{
			Use:   "version",
			Short: "Print the version number of Arduino App CLI",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("Arduino App CLI v" + Version)
			},
		},
	)

	ctx := context.Background()
	ctx, _ = cleanup.InterruptableContext(ctx)
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		slog.Error(err.Error())
	}
}
