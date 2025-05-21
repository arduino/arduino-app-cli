package orchestrator

import (
	"os"

	"github.com/arduino/go-paths-helper"
)

type OrchestratorConfig struct {
	appsDir *paths.Path
	dataDir *paths.Path
}

func NewOrchestratorConfigFromEnv() (*OrchestratorConfig, error) {
	appsDir := paths.New(os.Getenv("ARDUINO_APP_CLI__APPS_DIR"))
	if appsDir == nil {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		appsDir = paths.New(home).Join("arduino-apps")
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
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dataDir = paths.New(home).Join(".arduino-app-cli")
	}
	if !dataDir.IsAbs() {
		wd, err := paths.Getwd()
		if err != nil {
			return nil, err
		}
		dataDir = wd.JoinPath(dataDir)
	}

	c := &OrchestratorConfig{
		appsDir: appsDir,
		dataDir: dataDir,
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

func DefaultOrchestratorConfig() *OrchestratorConfig {
	return orchestratorConfig
}
