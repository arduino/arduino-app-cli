package main

import (
	"embed"
	"fmt"
	"log/slog"
	"net/url"
	"os"

	"github.com/arduino/arduino-app-cli/pkg/autoupdater/releaser"

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
		updateURL = "http://127.0.0.1:3001/"
	}

	parsedURL, err := url.Parse(updateURL)
	if err != nil {
		fmt.Println("Invalid UPDATE_URL:", err)
		os.Exit(1)
	}

	fmt.Println("Current Version: ", version)

	headers := map[string]string{}
	clientID := os.Getenv("CF_ACCESS_CLIENT_ID")
	clientSecret := os.Getenv("CF_ACCESS_CLIENT_SECRET")
	if clientID != "" && clientSecret != "" {
		headers["CF-Access-Client-Id"] = clientID
		headers["CF-Access-Client-Secret"] = clientSecret
	}

	slog.Info("Starting App", "version", version, "updateURL", updateURL, "clientID", clientID)

	var client *releaser.Client
	if len(headers) > 0 {
		client = releaser.NewClient(parsedURL, "AppLab/Stable", releaser.WithHeaders(headers))
	} else {
		client = releaser.NewClient(parsedURL, "AppLab/Stable")
	}

	app := NewApp(client)

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
