// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newStartCmd(cfg config.Configuration) *cobra.Command {
	var verbose bool
	var noPrepare bool
	cmd := &cobra.Command{
		Use:   "start app_path",
		Short: "Start an Arduino App",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			app, err := Load(args[0])
			if err != nil {
				return err
			}
			return startHandler(cmd.Context(), cfg, app, verbose, !noPrepare)
		},
		ValidArgsFunction: completion.ApplicationNamesWithFilterFunc(cfg, func(apps orchestrator.AppInfo) bool {
			return apps.Status != orchestrator.StatusStarting &&
				apps.Status != orchestrator.StatusRunning
		}),
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	cmd.Flags().BoolVar(&noPrepare, "no-prepare", false, "Start the app without preparing it first, failing if its containers or models are not already on the board")

	return cmd
}

func startHandler(ctx context.Context, cfg config.Configuration, app app.ArduinoApp, verbose bool, prepare bool) error {
	out, _, getResult := feedback.OutputStreams()

	streamCb := func(message orchestrator.StreamMessage) {
		switch message.GetType() {
		case orchestrator.ProgressType:
			fmt.Fprintf(out, "Progress[%s]: %.0f%%\n", message.GetProgress().Name, message.GetProgress().Progress)
		case orchestrator.InfoType:
			fmt.Fprintln(out, "[INFO]", message.GetData())
		}
	}

	// A release is prepared apart, by the wrapper of a prepare, before it is started.
	// An editable app is prepared by its own start instead.
	if prepare && app.IsRelease() {
		if err := orchestrator.PrepareInstalledRelease(
			ctx,
			servicelocator.GetDockerClient(),
			servicelocator.GetProvisioner(),
			app,
			cfg,
			servicelocator.GetPlatform(),
			streamCb,
		); err != nil {
			feedback.Fatal(fmt.Sprintf("[ERROR] %s", err.Error()), feedback.ErrGeneric)
		}
	}

	err := orchestrator.StartApp(
		ctx,
		servicelocator.GetDockerClient(),
		servicelocator.GetProvisioner(),
		servicelocator.GetModelsIndex(),
		servicelocator.GetBricksIndex(),
		servicelocator.GetServicesIndex(),
		app,
		cfg,
		servicelocator.GetPlatform(),
		verbose,
		prepare,
		streamCb,
	)
	if err != nil {
		feedback.Fatal(decorateErrorMessage(err, app.FullPath.String()), feedback.ErrGeneric)
	}

	outputResult := getResult()
	feedback.PrintResult(startAppResult{
		AppName: app.Name,
		Status:  "started",
		Output:  outputResult,
	})

	return nil
}

func decorateErrorMessage(err error, appPath string) string {
	suggestions := missingVariableSuggestions(err, appPath)
	messages := make([]string, 0, len(suggestions)+1)
	messages = append(messages, fmt.Sprintf("[ERROR] %s", err))
	messages = append(messages, suggestions...)
	return strings.Join(messages, "\n")
}

// missingVariableSuggestions returns a list of suggestions for missing required environment variables
// if the given error contains missing required variable errors.
func missingVariableSuggestions(err error, appPath string) []string {
	var suggestions []string
	seen := map[string]struct{}{}
	var visit func(error)
	visit = func(err error) {
		if err == nil {
			return
		}
		if missing, ok := err.(*orchestrator.MissingRequiredVariableError); ok {
			suggestion := fmt.Sprintf("  arduino-app-cli app brick config %s %s %s=value", appPath, missing.BrickID, missing.Name)
			if _, found := seen[suggestion]; !found {
				seen[suggestion] = struct{}{}
				suggestions = append(suggestions, suggestion)
			}
			return
		}
		// errors.As stops at the first match. Visit every wrapped error so each
		// missing variable in an errors.Join gets its own configuration hint.
		switch err := err.(type) {
		case interface{ Unwrap() []error }:
			for _, wrapped := range err.Unwrap() {
				visit(wrapped)
			}
		case interface{ Unwrap() error }:
			visit(err.Unwrap())
		}
	}
	visit(err)
	if len(suggestions) > 0 {
		suggestions = append([]string{"To configure the variables use:"}, suggestions...)
	}
	return suggestions
}

type startAppResult struct {
	AppName string                        `json:"appName"`
	Status  string                        `json:"status"`
	Output  *feedback.OutputStreamsResult `json:"output,omitempty"`
}

func (r startAppResult) String() string {
	return fmt.Sprintf("✓ App %q started successfully", r.AppName)
}

func (r startAppResult) Data() any {
	return r
}
