package orchestrator

import (
	"log/slog"
	"os"

	"github.com/arduino/go-paths-helper"
)

type OrchestratorConfig struct {
	appsDir           *paths.Path
	dataDir           *paths.Path
	routerSocketPath  *paths.Path
	customEIModelsDir *paths.Path
}

func NewOrchestratorConfigFromEnv() (*OrchestratorConfig, error) {
	appsDir := paths.New(os.Getenv("ARDUINO_APP_CLI__APPS_DIR"))
	if appsDir == nil {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		appsDir = paths.New(home).Join("ArduinoApps")
	}

	if !appsDir.IsAbs() {
		wd, err := paths.Getwd()
		if err != nil {
			return nil, err
		}
		appsDir = wd.JoinPath(appsDir)
	}

	dataDir := paths.New(os.Getenv("ARDUINO_APP_CLI__DATA_DIR"))
	if dataDir == nil {
		xdgConfig, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}
		dataDir = paths.New(xdgConfig).Join("arduino-app-cli")
	}
	if !dataDir.IsAbs() {
		wd, err := paths.Getwd()
		if err != nil {
			return nil, err
		}
		dataDir = wd.JoinPath(dataDir)
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
			return nil, err
		}
		customEIModelsDir = paths.New(homeDir, ".arduino-bricks/ei-models")
	}
	if customEIModelsDir.NotExist() {
		if err := customEIModelsDir.MkdirAll(); err != nil {
			slog.Warn("failed create custom model directory", "error", err)
		}
	}

	c := &OrchestratorConfig{
		appsDir:           appsDir,
		dataDir:           dataDir,
		routerSocketPath:  routerSocket,
		customEIModelsDir: customEIModelsDir,
	}
	if err := c.init(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *OrchestratorConfig) init() error {
	if err := c.AppsDir().MkdirAll(); err != nil {
		return err
	}
	if err := c.dataDir.MkdirAll(); err != nil {
		return err
	}
	if err := c.ExamplesDir().MkdirAll(); err != nil {
		return err
	}
	return nil
}

func (c *OrchestratorConfig) AppsDir() *paths.Path {
	return c.appsDir
}

func (c *OrchestratorConfig) DataDir() *paths.Path {
	return c.dataDir
}

func (c *OrchestratorConfig) ExamplesDir() *paths.Path {
	return c.dataDir.Join("examples")
}

func (c *OrchestratorConfig) RouterSocketPath() *paths.Path {
	return c.routerSocketPath
}

type ConfigResponse struct {
	Directories ConfigDirectories `json:"directories"`
}

type ConfigDirectories struct {
	Data     string `json:"data"`
	Apps     string `json:"apps"`
	Examples string `json:"examples"`
}

func GetOrchestratorConfig() ConfigResponse {
	return ConfigResponse{
		Directories: ConfigDirectories{
			Data:     orchestratorConfig.DataDir().String(),
			Apps:     orchestratorConfig.AppsDir().String(),
			Examples: orchestratorConfig.ExamplesDir().String(),
		},
	}
}
