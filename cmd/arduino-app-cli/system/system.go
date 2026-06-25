// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/docker/cli/cli/command"
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/cmdutil"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/helpers"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/update"
	"github.com/arduino/arduino-app-cli/internal/update/apt"
	"github.com/arduino/arduino-app-cli/internal/update/arduino"
	"github.com/arduino/arduino-app-cli/pkg/board"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/local"
)

func NewSystemCmd(cfg config.Configuration) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "Manage the board’s system configuration",
	}

	cmd.AddCommand(newDownloadImageCmd(cfg))
	cmd.AddCommand(newUpdateCmd(cfg))
	cmd.AddCommand(newCleanUpCmd(cfg, servicelocator.GetDockerClient()))
	cmd.AddCommand(newNetworkModeCmd())
	cmd.AddCommand(newKeyboardSetCmd())
	cmd.AddCommand(newBoardSetNameCmd())

	return cmd
}

// jsonInitEvent is the JSON-lines representation of a single event emitted by
// `system init --json-feedback`. The Type field ("log" | "progress") is a
// discriminator that tells the consumer which fields are meaningful.
type jsonInitEvent struct {
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
	Label   string `json:"label,omitempty"`
	Current int64  `json:"current,omitempty"`
	Total   int64  `json:"total,omitempty"`
	Percent int    `json:"percent,omitempty"`
}

// newInitEventCallback builds the single callback that renders SystemInit events
// to stdout. The same events are emitted regardless of the flag; only the
// formatting differs (plain text lines vs JSON lines). Progress events are
// throttled so consecutive updates with the same integer percentage are dropped.
func newInitEventCallback(stdout io.Writer, jsonFeedback bool) orchestrator.InitEventCallback {
	var render orchestrator.InitEventCallback
	if jsonFeedback {
		// One JSON object per line (JSONL). json.Encoder.Encode writes a trailing
		// newline after each object, which is exactly the framing a line-based
		// consumer needs.
		enc := json.NewEncoder(stdout)
		render = func(e orchestrator.InitEvent) {
			switch e.Type {
			case orchestrator.InitLogEvent:
				_ = enc.Encode(jsonInitEvent{Type: "log", Message: e.Message})
			case orchestrator.InitProgressEvent:
				p := e.Progress
				var percentage int
				if p.Total > 0 {
					percentage = int(float64(p.Curr) / float64(p.Total) * 100)
				}
				_ = enc.Encode(jsonInitEvent{
					Type:    "progress",
					Label:   p.Label,
					Current: p.Curr,
					Total:   p.Total,
					Percent: percentage,
				})
			}
		}
	} else {
		render = func(e orchestrator.InitEvent) {
			switch e.Type {
			case orchestrator.InitLogEvent:
				fmt.Fprintln(stdout, e.Message)
			case orchestrator.InitProgressEvent:
				p := e.Progress
				percentage := float64(p.Curr) / float64(p.Total) * 100
				fmt.Fprintf(stdout, "%s: %.0f%%\n", p.Label, percentage)
			}
		}
	}
	return throttleProgress(render)
}

// throttleProgress wraps an event callback so that consecutive progress events
// for the same label are only forwarded when their integer percentage changes.
// Log events always pass through. This bounds the output volume (~100 progress
// lines per label) without dropping any log line, identically in both formats.
func throttleProgress(next orchestrator.InitEventCallback) orchestrator.InitEventCallback {
	lastPct := map[string]int{}
	return func(e orchestrator.InitEvent) {
		if e.Type == orchestrator.InitProgressEvent {
			p := e.Progress
			if p.Total <= 0 {
				return
			}
			pct := int(float64(p.Curr) / float64(p.Total) * 100)
			if last, ok := lastPct[p.Label]; ok && last == pct {
				return
			}
			lastPct[p.Label] = pct
		}
		next(e)
	}
}

func newDownloadImageCmd(cfg config.Configuration) *cobra.Command {
	var onlyImages bool
	var onlyPlatformAndLibraries bool
	var jsonFeedback bool
	cmd := &cobra.Command{
		Use:    "init",
		Args:   cobra.ExactArgs(0),
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			stdout, _, err := feedback.DirectStreams()
			if err != nil {
				return err
			}
<<<<<<< HEAD
			progressCB := func(progress orchestrator.InitProgress) {
				percentage := float64(progress.Curr) / float64(progress.Total) * 100
				fmt.Fprintf(stdout, "%s: %.2f%% (%d/%d)\r", progress.Label, percentage, progress.Curr, progress.Total)
				if progress.Curr == progress.Total {
					fmt.Fprintln(stdout)
				}
			}
			return orchestrator.SystemInit(cmd.Context(), cfg, servicelocator.GetPlatform(), servicelocator.GetBricksIndex(), servicelocator.GetServicesIndex(), servicelocator.GetDockerClient(), servicelocator.GetModelsIndex(), orchestrator.SystemInitOptions{
=======

			eventCB := newInitEventCallback(stdout, jsonFeedback)
			return orchestrator.SystemInit(cmd.Context(), cfg, servicelocator.GetPlatform(), servicelocator.GetBricksIndex(), servicelocator.GetServicesIndex(), servicelocator.GetDockerClient(), orchestrator.SystemInitOptions{
>>>>>>> ede498ed2 (add json output and refactoring)
				OnlyDockerImages:    onlyImages,
				OnlyPlatformAndLibs: onlyPlatformAndLibraries,
			}, eventCB)
		},
	}

	cmd.PersistentFlags().BoolVar(&onlyImages, "only-docker-images", false, "Only download the application docker images")
	cmd.PersistentFlags().BoolVar(&onlyPlatformAndLibraries, "only-arduino-platform", false, "Only download the Arduino platform and libraries")
	cmd.PersistentFlags().BoolVar(&jsonFeedback, "json-feedback", false, "Emit logs and progress as JSON lines (one event per line) instead of plain text")

	return cmd
}

func newUpdateCmd(cfg config.Configuration) *cobra.Command {
	var onlyArduino bool
	var forceYes bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Launches an update of the upgradable packages on the system",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			filterFunc := getFilterFunc(onlyArduino)

			updater := update.NewManager(
				apt.New(),
				arduino.NewArduinoPlatformUpdater(servicelocator.GetPlatform(), cfg.ArduinoPlatformVersionConstraint),
			)

			pkgs, err := updater.ListUpgradablePackages(cmd.Context(), filterFunc)
			if err != nil {
				return err
			}
			if len(pkgs) == 0 {
				feedback.Printf("No upgradable packages found.")
				return nil
			}

			feedback.Printf("Found %d upgradable packages:", len(pkgs))
			for _, pkg := range pkgs {
				feedback.Printf("Package: %s, From: %s, To: %s", pkg.Name, pkg.FromVersion, pkg.ToVersion)
			}

			feedback.Printf("Do you want to upgrade these packages? (yes/no)")
			var yes bool
			if forceYes {
				yes = true
			} else {
				var yesInput string
				_, err := fmt.Scanf("%s\n", &yesInput)
				if err != nil {
					return err
				}
				yes = strings.ToLower(yesInput) == "yes" || strings.ToLower(yesInput) == "y"
			}

			if !yes {
				return nil
			}

			if err := updater.UpgradePackages(cmd.Context(), pkgs); err != nil {
				return err
			}

			events := updater.Subscribe()
			for event := range events {
				if event.Type == update.ErrorEvent {
					// TODO: add colors to error messages
					err := event.GetError()
					feedback.Printf("Error: %s [%s]", err.Error(), update.GetUpdateErrorCode(err))
				} else {
					feedback.Printf("[%s] %s", event.Type.String(), event.GetData())
				}

				if event.Type == update.DoneEvent {
					break
				}
			}
			return nil
		},
	}

	cmd.PersistentFlags().BoolVar(&onlyArduino, "only-arduino", false, "Only upgrades Arduino specific packages")
	cmd.PersistentFlags().BoolVar(&forceYes, "yes", false, "Automatically confirm all prompts")

	return cmd
}

func getFilterFunc(onlyArduino bool) func(p update.UpgradablePackage) bool {
	if onlyArduino {
		return update.MatchArduinoPackage
	}
	return update.MatchAllPackages
}

func newCleanUpCmd(cfg config.Configuration, docker command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Removes unused and obsolete application images to free up disk space.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			feedback.Printf("Running cleanup...")
			result, err := orchestrator.SystemCleanup(cmd.Context(), cfg, servicelocator.GetBricksIndex(), servicelocator.GetServicesIndex(), servicelocator.GetModelsIndex(), docker, servicelocator.GetPlatform())
			if err != nil {
				return err
			}

			if result.IsEmpty() {
				feedback.Print("Nothing to clean up.")
				return nil
			}

			feedback.Print("Cleanup successful.")
			feedback.Print("Freed up")
			if result.RunningAppRemoved {
				feedback.Print("  - 1 running app")
			}
			feedback.Printf("  - %d containers", result.ContainersRemoved)
			feedback.Printf("  - %d images (%v)", result.ImagesRemoved, helpers.ToHumanMiB(result.SpaceFreed))
			feedback.Printf("  - %d networks", result.NetworksRemoved)
			return nil
		},
	}
	return cmd
}

func newNetworkModeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network-mode <enable|disable|status>",
		Short: "Manage the network mode of the system",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "enable":
				pass, err := cmdutil.AskForPassword()
				if err != nil {
					return fmt.Errorf("failed to read password: %w", err)
				}
				if err := board.EnableNetworkMode(&local.LocalConnection{}, pass); err != nil {
					return fmt.Errorf("failed to enable network mode: %w", err)
				}

				feedback.Printf("network mode enabled and started")
				return nil
			case "disable":
				pass, err := cmdutil.AskForPassword()
				if err != nil {
					return fmt.Errorf("failed to read password: %w", err)
				}
				if err := board.DisableNetworkMode(&local.LocalConnection{}, pass); err != nil {
					return fmt.Errorf("failed to disable network mode: %w", err)
				}
				feedback.Printf("network mode disabled and stopped")
				return nil
			case "status":
				if isEnabled, err := board.NetworkModeStatus(cmd.Context(), &local.LocalConnection{}); err != nil {
					return fmt.Errorf("failed to check network mode status: %w", err)
				} else {
					if isEnabled {
						feedback.Printf("enabled")
					} else {
						feedback.Printf("disabled")
					}
				}
				return nil
			default:
				return fmt.Errorf("invalid argument: %s", args[0])
			}
		}}

	return cmd
}

func newKeyboardSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keyboard [layout]",
		Short: "Manage the keyboard layout of the system",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			layouts, err := board.ListKeyboardLayouts(&local.LocalConnection{})
			if err != nil {
				return fmt.Errorf("failed to list keyboard layouts: %w", err)
			}

			if len(args) == 0 {
				feedback.Printf("available layouts:")
				for _, l := range layouts {
					feedback.Printf("  - %s: %s", l.LayoutId, l.Description)
				}
				layout, err := board.GetKeyboardLayout(cmd.Context(), &local.LocalConnection{})
				if err != nil {
					return fmt.Errorf("failed to get keyboard layout: %w", err)
				}
				feedback.Printf("\ncurrent layout: %s", layout)
			} else {
				layout := args[0]

				if !slices.ContainsFunc(layouts, func(l board.KeyboardLayout) bool {
					return l.LayoutId == layout
				}) {
					return fmt.Errorf("invalid layout code: %s", layout)
				}

				if err := board.SetKeyboardLayout(cmd.Context(), &local.LocalConnection{}, layout); err != nil {
					return fmt.Errorf("failed to set keyboard layout: %w", err)
				}
				feedback.Printf("keyboard layout set to %s", layout)
			}

			return nil
		}}

	return cmd
}

func newBoardSetNameCmd() *cobra.Command {
	setNameCmd := &cobra.Command{
		Use:   "set-name <name>",
		Short: "Set the custom name of the board",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := board.SetCustomName(cmd.Context(), &local.LocalConnection{}, name); err != nil {
				return fmt.Errorf("failed to set custom name: %w", err)
			}
			feedback.Printf("Custom name set to %q\n", name)
			return nil
		},
	}

	return setNameCmd
}
