package main

import (
	"embed"
	"fmt"
	"os"
	"os/exec"

	"github.com/arduino/arduino-create-agent/updater"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

var (
	version = "x.x.x-dev"
)

func main() {
	updateURL := os.Getenv("UPDATE_URL")
	if updateURL == "" {
		updateURL = "http://127.0.0.1:3001/" // fallback default
	}

	src, err := os.Executable()
	if err != nil {
		panic(err)
	}

	fmt.Printf("[%s] Current Version: %s\n", src, version)

	// NOTE: other Start is required to have the "-temp" binary copied at start up, otherwise the `update` command fails
	restartPath := updater.Start(src)
	if restartPath != "" {
		fmt.Println("Restarting with updated binary", restartPath)
		cmd := exec.Command(restartPath, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		err := cmd.Start()
		if err != nil {
			panic(err)
		}
		// here we do not have the context of the app, so we cannot call runtime.Quit
		os.Exit(0)
	} else {
		fmt.Println("Starting normally")
	}

	// Create an instance of the app structure
	app := NewApp(updateURL)

	// Create application with options
	err = wails.Run(&options.App{
		Title:  "test-wails",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
