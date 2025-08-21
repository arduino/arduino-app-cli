package config

import (
	"log/slog"
	"os"

	"github.com/arduino/go-paths-helper"
)

type Configuration struct {
	appsDir           *paths.Path
	configDir         *paths.Path
	dataDir           *paths.Path
	routerSocketPath  *paths.Path
	customEIModelsDir *paths.Path
}

func NewFromEnv() (Configuration, error) {
	appsDir := paths.New(os.Getenv("ARDUINO_APP_CLI__APPS_DIR"))
	if appsDir == nil {
		home, err := os.UserHomeDir()
		if err != nil {
			return Configuration{}, err
		}
		appsDir = paths.New(home).Join("ArduinoApps")
	}

	if !appsDir.IsAbs() {
		wd, err := paths.Getwd()
		if err != nil {
			return Configuration{}, err
		}
		appsDir = wd.JoinPath(appsDir)
	}

	configDir := paths.New(os.Getenv("ARDUINO_APP_CLI__CONFIG_DIR"))
	if configDir == nil {
		xdgConfig, err := os.UserConfigDir()
		if err != nil {
			return Configuration{}, err
		}
		configDir = paths.New(xdgConfig).Join("arduino-app-cli")
	}
	if !configDir.IsAbs() {
		wd, err := paths.Getwd()
		if err != nil {
			return Configuration{}, err
		}
		configDir = wd.JoinPath(configDir)
	}

	dataDir := paths.New(os.Getenv("ARDUINO_APP_CLI__DATA_DIR"))
	if dataDir == nil {
		xdgHome, err := os.UserHomeDir()
		if err != nil {
			return Configuration{}, err
		}
		dataDir = paths.New(xdgHome).Join(".local", "share", "arduino-app-cli")
	}

	routerSocket := paths.New(os.Getenv("ARDUINO_ROUTER_SOCKET"))
	if routerSocket == nil || routerSocket.NotExist() {
		routerSocket = paths.New("/var/run/arduino-router.sock")
	}

	// Ensure the custom EI modules directory exists
	customEIModelsDir := paths.New(os.Getenv("ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR"))
	if customEIModelsDir == nil {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return Configuration{}, err
		}
		customEIModelsDir = paths.New(homeDir, ".arduino-bricks/ei-models")
	}
	if customEIModelsDir.NotExist() {
		if err := customEIModelsDir.MkdirAll(); err != nil {
			slog.Warn("failed create custom model directory", "error", err)
		}
	}

	c := Configuration{
		appsDir:           appsDir,
		configDir:         configDir,
		dataDir:           dataDir,
		routerSocketPath:  routerSocket,
		customEIModelsDir: customEIModelsDir,
	}
	if err := c.init(); err != nil {
		return Configuration{}, err
	}
	return c, nil
}

func (c *Configuration) init() error {
	if err := c.AppsDir().MkdirAll(); err != nil {
		return err
	}
	if err := c.ExamplesDir().MkdirAll(); err != nil {
		return err
	}
	if err := c.AssetsDir().MkdirAll(); err != nil {
		return err
	}
	return nil
}

func (c *Configuration) AppsDir() *paths.Path {
	return c.appsDir
}

func (c *Configuration) ConfigDir() *paths.Path {
	return c.configDir
}

func (c *Configuration) DataDir() *paths.Path {
	return c.dataDir
}

func (c *Configuration) ExamplesDir() *paths.Path {
	return c.dataDir.Join("examples")
}

func (c *Configuration) RouterSocketPath() *paths.Path {
	return c.routerSocketPath
}

func (c *Configuration) AssetsDir() *paths.Path {
	return c.dataDir.Join("assets")
}
