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

package system

import (
	"fmt"
	"slices"
	"strings"

	"github.com/docker/cli/cli/command"
	"github.com/spf13/cobra"

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
	cmd.AddCommand(newUpdateCmd())
	cmd.AddCommand(newCleanUpCmd(cfg, servicelocator.GetDockerClient()))
	cmd.AddCommand(newNetworkModeCmd())
	cmd.AddCommand(newKeyboardSetCmd())
	cmd.AddCommand(newBoardSetNameCmd())

	return cmd
}

func newDownloadImageCmd(cfg config.Configuration) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "init",
		Args:   cobra.ExactArgs(0),
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return orchestrator.SystemInit(cmd.Context(), cfg, servicelocator.GetStaticStore(), servicelocator.GetDockerClient())
		},
	}

	return cmd
}

func newUpdateCmd() *cobra.Command {
	var onlyArduino bool
	var forceYes bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Launches an update of the upgradable packages on the system",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			filterFunc := getFilterFunc(onlyArduino)

			updater := getUpdater()

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
				feedback.Printf("[%s] %s", event.Type.String(), event.Data)

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

func getUpdater() *update.Manager {
	return update.NewManager(
		apt.New(),
		arduino.NewArduinoPlatformUpdater(),
	)
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
			staticStore := servicelocator.GetStaticStore()

			feedback.Printf("Running cleanup...")
			result, err := orchestrator.SystemCleanup(cmd.Context(), cfg, staticStore, docker)
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
				if err := board.EnableNetworkMode(cmd.Context(), &local.LocalConnection{}); err != nil {
					return fmt.Errorf("failed to enable network mode: %w", err)
				}

				feedback.Printf("network mode enabled and started")
			case "disable":
				if err := board.DisableNetworkMode(cmd.Context(), &local.LocalConnection{}); err != nil {
					return fmt.Errorf("failed to disable network mode: %w", err)
				}
				feedback.Printf("network mode disabled and stopped")
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
			}

			return nil
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
