package main

import (
	"context"
	"log/slog"

	dockerClient "github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"go.bug.st/cleanup"
)

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
	}

	rootCmd.AddCommand(
		newAppCmd(docker),
		newCompletionCommand(),
		newDaemonCmd(docker),
	)

	ctx := context.Background()
	ctx, _ = cleanup.InterruptableContext(ctx)
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		slog.Error(err.Error())
	}
}
