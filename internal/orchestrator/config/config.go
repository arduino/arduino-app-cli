package config

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/arduino/go-paths-helper"
)

// runnerVersion do not edit, this is generate with `task generate:assets`
var runnerVersion = "0.4.5"

type Configuration struct {
	appsDir            *paths.Path
	configDir          *paths.Path
	dataDir            *paths.Path
	routerSocketPath   *paths.Path
	customEIModelsDir  *paths.Path
	PythonImage        string
	UsedPythonImageTag string
	RunnerVersion      string
	AllowRoot          bool
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

	pythonImage, usedPythonImageTag := getPythonImageAndTag()
	slog.Debug("Using pythonImage", slog.String("image", pythonImage))

	allowRoot, err := strconv.ParseBool(os.Getenv("ARDUINO_APP_CLI__ALLOW_ROOT"))
	if err != nil {
		allowRoot = false
	}

	c := Configuration{
		appsDir:            appsDir,
		configDir:          configDir,
		dataDir:            dataDir,
		routerSocketPath:   routerSocket,
		customEIModelsDir:  customEIModelsDir,
		PythonImage:        pythonImage,
		UsedPythonImageTag: usedPythonImageTag,
		RunnerVersion:      runnerVersion,
		AllowRoot:          allowRoot,
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

func getPythonImageAndTag() (string, string) {
	registryBase := os.Getenv("DOCKER_REGISTRY_BASE")
	if registryBase == "" {
		registryBase = "public.ecr.aws/arduino/"
	}

	// Python image: image name (repository) and optionally a tag.
	pythonImageAndTag := os.Getenv("DOCKER_PYTHON_BASE_IMAGE")
	if pythonImageAndTag == "" {
		pythonImageAndTag = fmt.Sprintf("app-bricks/python-apps-base:%s", runnerVersion)
	}
	pythonImage := path.Join(registryBase, pythonImageAndTag)
	var usedPythonImageTag string
	if idx := strings.LastIndex(pythonImage, ":"); idx != -1 {
		usedPythonImageTag = pythonImage[idx+1:]
	}
	return pythonImage, usedPythonImageTag
}
