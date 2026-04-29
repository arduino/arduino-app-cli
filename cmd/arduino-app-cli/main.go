// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
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
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/model"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/monitor"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/properties"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/system"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/version"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/cmd/i18n"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
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
		Short: "A CLI to manage Arduino Apps",
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
		brick.NewBrickCmd(configuration),
		completion.NewCompletionCommand(),
		daemon.NewDaemonCmd(configuration, Version),
		properties.NewPropertiesCmd(configuration),
		config.NewConfigCmd(configuration),
		system.NewSystemCmd(configuration),
		version.NewVersionCmd(Version),
		monitor.NewMonitorCmd(),
		model.NewModelCmd(configuration),
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

	if os.Geteuid() != 1000 && !configuration.AllowRoot {
		feedback.Fatal("arduino-app-cli must be run as a non-root user with UID 1000. Try `su - arduino` before this command.", feedback.ErrGeneric)
	}

	if err := run(configuration); err != nil {
		if errors.Is(err, orchestrator.ErrDockerOutOfSpace) {
			// Return a specific error code in case a specific error happened (disk full when pulling docker images).
			feedback.FatalError(err, orchestrator.ExitCodeDockerOutOfSpace)
		}
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
