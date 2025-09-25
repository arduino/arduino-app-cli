package system

import (
	"fmt"
	"slices"
	"strings"

	"github.com/docker/cli/cli/command"
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/update"
	"github.com/arduino/arduino-app-cli/internal/update/apt"
	"github.com/arduino/arduino-app-cli/internal/update/arduino"
	"github.com/arduino/arduino-app-cli/pkg/board"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/local"
	"github.com/arduino/arduino-app-cli/pkg/x"
)

func NewSystemCmd(cfg config.Configuration) *cobra.Command {
	cmd := &cobra.Command{
		Use: "system",
	}

	cmd.AddCommand(newDownloadImage(cfg))
	cmd.AddCommand(newUpdateCmd())
	cmd.AddCommand(newCleanUpCmd(cfg, servicelocator.GetDockerClient()))
	cmd.AddCommand(newNetworkMode())
	cmd.AddCommand(newkeyboardSet())

	return cmd
}

func newDownloadImage(cfg config.Configuration) *cobra.Command {
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
			feedback.Printf("  - %d images (%v)", result.ImagesRemoved, x.ToHumanMiB(result.SpaceFreed))
			return nil
		},
	}
	return cmd
}

func newNetworkMode() *cobra.Command {
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

func newkeyboardSet() *cobra.Command {
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
