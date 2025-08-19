package main

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"go.bug.st/cleanup"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/app"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/board"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/brick"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/config"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/daemon"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/properties"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/system"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/version"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/cmd/i18n"
	cfg "github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

// Version will be set a build time with -ldflags
var Version string = "0.0.0-dev"
var format string

func run(configuration cfg.Configuration) error {
	servicelocator.Init(configuration)
	defer func() { _ = servicelocator.CloseDockerClient() }()

	logLevel, err := ParseLogLevel(cmp.Or(os.Getenv("ARDUINO_APP_CLI__LOG_LEVEL"), "INFO"))
	if err != nil {
		return err
	}
	slog.SetLogLoggerLevel(logLevel)

	rootCmd := &cobra.Command{
		Use:   "arduino-app-cli",
		Short: "A CLI to manage the Python app",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			format, ok := feedback.ParseOutputFormat(format)
			if !ok {
				feedback.Fatal(i18n.Tr("Invalid output format: %s", format), feedback.ErrBadArgument)
			}
			feedback.SetFormat(format)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().StringVar(&format, "format", "text", "Output format (text, json)")

	rootCmd.AddCommand(
		app.NewAppCmd(configuration),
		brick.NewBrickCmd(),
		completion.NewCompletionCommand(),
		daemon.NewDaemonCmd(configuration, Version),
		properties.NewPropertiesCmd(configuration),
		config.NewConfigCmd(configuration),
		system.NewSystemCmd(),
		board.NewBoardCmd(),
		version.NewVersionCmd(Version),
	)

	ctx := context.Background()
	ctx, _ = cleanup.InterruptableContext(ctx)
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		return err
	}

	return nil
}

func main() {
	configuration, err := cfg.NewFromEnv()
	if err != nil {
		feedback.Fatal(fmt.Sprintf("invalid config: %s", err), feedback.ErrGeneric)
	}
	if err := run(configuration); err != nil {
		feedback.FatalError(err, 1)
	}
}

func ParseLogLevel(level string) (slog.Level, error) {
	var l slog.Level
	err := l.UnmarshalText([]byte(level))
	if err != nil {
		return 0, fmt.Errorf("invalid log level: %w", err)
	}
	return l, nil
}
