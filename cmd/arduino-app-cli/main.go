// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"go.bug.st/cleanup"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/app"
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
var logLevelStr string

func run(configuration cfg.Configuration) error {
	servicelocator.Init(configuration)
	defer func() { _ = servicelocator.CloseDockerClient() }()
	rootCmd := &cobra.Command{
		Use:   "arduino-app-cli",
		Short: "A CLI to manage the Python app",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			format, ok := feedback.ParseOutputFormat(format)
			if !ok {
				feedback.Fatal(i18n.Tr("Invalid output format: %s", format), feedback.ErrBadArgument)
			}
			feedback.SetFormat(format)

			logLevel, err := ParseLogLevel(logLevelStr)
			if err != nil {
				feedback.FatalError(err, feedback.ErrBadArgument)
			}
			slog.SetLogLoggerLevel(logLevel)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().StringVar(&format, "format", "text", "Output format (text, json)")
	rootCmd.PersistentFlags().StringVar(&logLevelStr, "log-level", "error", "Set the log level (debug, info, warn, error)")

	rootCmd.AddCommand(
		app.NewAppCmd(configuration),
		brick.NewBrickCmd(),
		completion.NewCompletionCommand(),
		daemon.NewDaemonCmd(configuration, Version),
		properties.NewPropertiesCmd(configuration),
		config.NewConfigCmd(configuration),
		system.NewSystemCmd(configuration),
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

	if os.Geteuid() == 0 && !configuration.AllowRoot {
		feedback.Fatal("arduino-app-cli must not be run as root. Try `su - arduino` before this command.", feedback.ErrGeneric)
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
