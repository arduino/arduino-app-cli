package main

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"go.bug.st/cleanup"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
)

// Version will be set a build time with -ldflags
var Version string = "0.0.0-dev"

func main() {
	defer func() { _ = servicelocator.CloseDockerClient() }()

	logLevel := ParseLogLevel(cmp.Or(os.Getenv("ARDUINO_APP_CLI__LOG_LEVEL"), "INFO"))
	slog.SetLogLoggerLevel(logLevel)

	rootCmd := &cobra.Command{
		Use:   "arduino-app-cli",
		Short: "A CLI to manage the Python app",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.AddCommand(
		newAppCmd(),
		newBrickCmd(),
		newCompletionCommand(),
		newDaemonCmd(),
		newPropertiesCmd(),
		newConfigCmd(),
		newSystemCmd(),
		&cobra.Command{
			Use:   "version",
			Short: "Print the version number of Arduino App CLI",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("Arduino App CLI v" + Version)
			},
		},
		newFSCmd(),
	)

	ctx := context.Background()
	ctx, _ = cleanup.InterruptableContext(ctx)
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		slog.Error(err.Error())
	}
}

func ParseLogLevel(level string) slog.Level {
	var l slog.Level
	err := l.UnmarshalText([]byte(level))
	if err != nil {
		fmt.Printf("Invalid log level: %s\n", level)
		os.Exit(1)
	}
	return l
}
