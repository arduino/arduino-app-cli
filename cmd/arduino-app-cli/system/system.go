package system

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/update"
	"github.com/arduino/arduino-app-cli/internal/update/apt"
	"github.com/arduino/arduino-app-cli/internal/update/arduino"
)

func NewSystemCmd(cfg config.Configuration) *cobra.Command {
	cmd := &cobra.Command{
		Use: "system",
	}

	cmd.AddCommand(newDownloadImage(cfg))
	cmd.AddCommand(newUpdateCmd())

	return cmd
}

func newDownloadImage(cfg config.Configuration) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "init",
		Args:   cobra.ExactArgs(0),
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return orchestrator.SystemInit(cmd.Context(), cfg.UsedPythonImageTag, servicelocator.GetStaticStore())
		},
	}

	return cmd
}

func newUpdateCmd() *cobra.Command {
	var onlyArduino bool
	var forceYes bool
	cmd := &cobra.Command{
		Use:  "update",
		Args: cobra.ExactArgs(0),
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

	cmd.PersistentFlags().BoolVar(&onlyArduino, "only-arduino", false, "Check for all upgradable packages")
	cmd.PersistentFlags().BoolVar(&forceYes, "--yes", false, "Automatically confirm all prompts")

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
		return func(p update.UpgradablePackage) bool {
			return strings.HasPrefix(p.Name, "arduino-")
		}
	}
	return func(p update.UpgradablePackage) bool {
		return true
	}
}
