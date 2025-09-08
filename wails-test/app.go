package main

import (
	"context"
	"fmt"
	"os"

	"github.com/arduino/arduino-app-cli/pkg/autoupdater/releaser"
	"github.com/arduino/arduino-app-cli/pkg/autoupdater/updater"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx    context.Context
	client *releaser.Client
}

// NewApp creates a new App application struct
func NewApp(client *releaser.Client) *App {
	return &App{
		client: client,
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {
	a.ctx = ctx
	fmt.Println("Shutted down wails")
}

func (a *App) GetVersion() string {
	return version
}

func (a *App) CheckAndApplyUpdate() error {
	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not get executable path: %w", err)
	}
	upgradeConfirm := func(current releaser.Version, target releaser.Version) bool {
		result, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:          runtime.QuestionDialog,
			Title:         "Update available",
			Message:       "Do you want to upgrade from " + current.String() + " to " + target.String() + "?",
			Buttons:       []string{"Yes", "No", "Cancel"},
			DefaultButton: "Yes",
		})

		if err != nil {
			fmt.Println("Error showing dialog:", err)
			return false
		}

		if result == "Yes" {
			fmt.Println("User confirmed the action.")
			return true
		}

		return false

	}

	err = updater.CheckForUpdates(executablePath, releaser.Version(version), a.client, upgradeConfirm)
	if err != nil {
		return fmt.Errorf("Error checking for updates: %w", err)
	}

	_, err = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:    runtime.InfoDialog,
		Title:   "AppLab",
		Message: "There is no update available.",
	})
	return nil
}

func (a *App) GetLatestVersion() (string, error) {
	env := runtime.Environment(a.ctx)

	plat := releaser.NewPlatform(env.Platform, env.Arch)
	info, err := a.client.GetLatestVersion(plat)
	if err != nil {
		return "", err
	}

	return info.Version.String(), nil
}
