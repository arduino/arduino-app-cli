// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package config

import (
	"cmp"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/arduino/go-paths-helper"
	"go.bug.st/f"
	semver "go.bug.st/relaxed-semver"
)

// runnerVersion do not edit, this is generate with `task generate:assets`
var RunnerVersion = "0.8.0"

type Configuration struct {
	appsDir                          *paths.Path
	dataDir                          *paths.Path
	routerSocketPath                 *paths.Path
	customModelsDir                  *paths.Path
	PythonImage                      string
	AllowRoot                        bool
	LibrariesAPIURL                  *url.URL
	EdgeImpulseAPIURL                *url.URL
	ArduinoPlatformVersionConstraint semver.Constraint
	PlatformOverride                 PlatformOverride
}

type PlatformOverride struct {
	FQBN string
}

func New() (Configuration, error) {
	conf := getDefaultConfig()
	if err := conf.fromFile(); err != nil {
		return Configuration{}, fmt.Errorf("failed to load configuration from file: %w", err)
	}
	if err := conf.fromEnv(); err != nil {
		return Configuration{}, fmt.Errorf("failed to load configuration from environment: %w", err)
	}

	c, err := conf.Parse()
	if err != nil {
		return Configuration{}, fmt.Errorf("failed to parse configuration: %w", err)
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
	if err := c.CustomModelsDir().MkdirAll(); err != nil {
		return err
	}
	return nil
}

func (c *Configuration) AppsDir() *paths.Path {
	return c.appsDir
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

func (c *Configuration) CustomModelsDir() *paths.Path {
	return c.customModelsDir
}

func (c *Configuration) GetUsedPythonImageTag() string {
	var usedPythonImageTag string
	if idx := strings.LastIndex(c.PythonImage, ":"); idx != -1 {
		usedPythonImageTag = c.PythonImage[idx+1:]
	}
	return usedPythonImageTag

}

type ConfigurationParser struct {
	AppsDir                          string `json:"apps_dir"`
	DataDir                          string `json:"data_dir"`
	RouterSocketPath                 string `json:"-"`
	CustomModelsDir                  string `json:"custom_models_dir"`
	PythonImage                      string `json:"python_image"`
	AllowRoot                        bool   `json:"-"`
	LibrariesAPIURL                  string `json:"libraries_api_url"`
	EdgeImpulseAPIURL                string `json:"edge_impulse_api_url"`
	ArduinoPlatformVersionConstraint string `json:"arduino_platform_version_constraint"`
	PlatformOverride                 struct {
		FQBN string `json:"fqbn"`
	} `json:"platform_override"`
}

func getDefaultConfig() ConfigurationParser {
	return ConfigurationParser{
		AppsDir: func() string {
			return paths.New(f.Must(os.UserHomeDir())).Join("ArduinoApps").String()
		}(),
		DataDir:          "/var/lib/arduino-app-cli",
		RouterSocketPath: "/var/run/arduino-router.sock",
		CustomModelsDir: func() string {
			return paths.New(f.Must(os.UserHomeDir()), ".arduino-bricks/models").String()
		}(),
		PythonImage:                      fmt.Sprintf("ghcr.io/arduino/app-bricks/python-apps-base:%s", RunnerVersion),
		AllowRoot:                        false,
		LibrariesAPIURL:                  "https://api2.arduino.cc/libraries/v1/libraries",
		EdgeImpulseAPIURL:                "https://studio.edgeimpulse.com/v1",
		ArduinoPlatformVersionConstraint: "<1.0.0",
		PlatformOverride: struct {
			FQBN string `json:"fqbn"`
		}{},
	}
}

func (c ConfigurationParser) Parse() (Configuration, error) {
	librariesAPIURL, err := url.Parse(c.LibrariesAPIURL)
	if err != nil {
		return Configuration{}, fmt.Errorf("invalid LIBRARIES_API_URL: %w", err)
	}

	edgeImpulseAPIURL, err := url.Parse(c.EdgeImpulseAPIURL)
	if err != nil {
		return Configuration{}, fmt.Errorf("invalid EDGE_IMPULSE_API_URL: %w", err)
	}

	arduinoPlatformVersionConstraint, err := semver.ParseConstraint(c.ArduinoPlatformVersionConstraint)
	if err != nil {
		return Configuration{}, fmt.Errorf("invalid version constraint: %w", err)
	}

	return Configuration{
		appsDir:                          paths.New(c.AppsDir),
		dataDir:                          paths.New(c.DataDir),
		routerSocketPath:                 paths.New(c.RouterSocketPath),
		customModelsDir:                  paths.New(c.CustomModelsDir),
		PythonImage:                      c.PythonImage,
		AllowRoot:                        c.AllowRoot,
		LibrariesAPIURL:                  librariesAPIURL,
		EdgeImpulseAPIURL:                edgeImpulseAPIURL,
		ArduinoPlatformVersionConstraint: arduinoPlatformVersionConstraint,
		PlatformOverride:                 PlatformOverride(c.PlatformOverride),
	}, nil
}

func (c *ConfigurationParser) fromFile() error {
	configPath := paths.New(c.DataDir).Join("config.json")
	if configPath.NotExist() {
		return nil
	}
	f, err := configPath.Open()
	if err != nil {
		slog.Debug("failed to open data directory, will attempt to create it", "path", c.DataDir, "error", err)
		return nil
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(c); err != nil {
		return fmt.Errorf("failed to decode configuration file: %w", err)
	}

	return nil
}

func (c *ConfigurationParser) fromEnv() error {
	if dataDir := os.Getenv("ARDUINO_APP_CLI__DATA_DIR"); dataDir != "" {
		c.DataDir = dataDir
	}

	if appsDir := os.Getenv("ARDUINO_APP_CLI__APPS_DIR"); appsDir != "" {
		c.AppsDir = appsDir
	}

	if routerSocketPath := os.Getenv("ARDUINO_ROUTER_SOCKET"); routerSocketPath != "" {
		c.RouterSocketPath = routerSocketPath
	}

	if customModelsDir := os.Getenv("ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR"); customModelsDir != "" {
		c.CustomModelsDir = customModelsDir
	}

	registryBase := cmp.Or(os.Getenv("DOCKER_REGISTRY_BASE"), "ghcr.io/arduino/")
	pythonImageAndTag := cmp.Or(os.Getenv("DOCKER_PYTHON_BASE_IMAGE"), fmt.Sprintf("app-bricks/python-apps-base:%s", RunnerVersion))
	pythonImage := path.Join(registryBase, pythonImageAndTag)
	c.PythonImage = pythonImage
	slog.Debug("Using pythonImage", slog.String("image", c.PythonImage))

	if allowRoot := os.Getenv("ARDUINO_APP_CLI__ALLOW_ROOT"); allowRoot != "" {
		var err error
		c.AllowRoot, err = strconv.ParseBool(allowRoot)
		if err != nil {
			return fmt.Errorf("invalid value for ARDUINO_APP_CLI__ALLOW_ROOT: %w", err)
		}
	}

	if librariesAPIURL := os.Getenv("LIBRARIES_API_URL"); librariesAPIURL != "" {
		c.LibrariesAPIURL = librariesAPIURL
	}

	if edgeImpulseAPIURL := os.Getenv("EDGE_IMPULSE_API_URL"); edgeImpulseAPIURL != "" {
		c.EdgeImpulseAPIURL = edgeImpulseAPIURL
	}

	if constraintStr := os.Getenv("ARDUINO_APP_CLI__PLATFORM_VERSION_CONSTRAINT"); constraintStr != "" {
		c.ArduinoPlatformVersionConstraint = constraintStr
	}

	return nil
}
