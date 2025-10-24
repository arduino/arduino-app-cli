// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/arduino/go-paths-helper"
)

// runnerVersion do not edit, this is generate with `task generate:assets`
var runnerVersion = "0.5.0"

type Configuration struct {
	appsDir            *paths.Path
	dataDir            *paths.Path
	routerSocketPath   *paths.Path
	customEIModelsDir  *paths.Path
	PythonImage        string
	UsedPythonImageTag string
	RunnerVersion      string
	AllowRoot          bool
	LibrariesAPIURL    *url.URL
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

	librariesAPIURL := os.Getenv("LIBRARIES_API_URL")
	if librariesAPIURL == "" {
		librariesAPIURL = "https://api2.arduino.cc/libraries/v1/libraries"
	}
	parsedLibrariesURL, err := url.Parse(librariesAPIURL)
	if err != nil {
		return Configuration{}, fmt.Errorf("invalid LIBRARIES_API_URL: %w", err)
	}

	c := Configuration{
		appsDir:            appsDir,
		dataDir:            dataDir,
		routerSocketPath:   routerSocket,
		customEIModelsDir:  customEIModelsDir,
		PythonImage:        pythonImage,
		UsedPythonImageTag: usedPythonImageTag,
		RunnerVersion:      runnerVersion,
		AllowRoot:          allowRoot,
		LibrariesAPIURL:    parsedLibrariesURL,
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
		registryBase = "ghcr.io/arduino/"
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
