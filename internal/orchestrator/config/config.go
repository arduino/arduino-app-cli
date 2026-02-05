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
	"cmp"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/arduino/go-paths-helper"
	semver "go.bug.st/relaxed-semver"
)

// runnerVersion do not edit, this is generate with `task generate:assets`
var RunnerVersion = "0.7.0"

type Configuration struct {
	appsDir                          *paths.Path
	dataDir                          *paths.Path
	routerSocketPath                 *paths.Path
	customModelsDir                  *paths.Path
	PythonImage                      string
	UsedPythonImageTag               string
	RunnerVersion                    string
	AllowRoot                        bool
	LibrariesAPIURL                  *url.URL
	EdgeImpulseAPIURL                *url.URL
	ArduinoPlatformVersionConstraint semver.Constraint
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
		dataDir = paths.New("/var/lib/arduino-app-cli")
	}

	routerSocket := paths.New(os.Getenv("ARDUINO_ROUTER_SOCKET"))
	if routerSocket == nil || routerSocket.NotExist() {
		routerSocket = paths.New("/var/run/arduino-router.sock")
	}

	// Ensure the custom modules directory exists
	customModelsDir := paths.New(os.Getenv("ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR"))
	if customModelsDir == nil {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return Configuration{}, err
		}
		customModelsDir = paths.New(homeDir, ".arduino-bricks/models")
	}
	if customModelsDir.NotExist() {
		if err := customModelsDir.MkdirAll(); err != nil {
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

	constraintStr := cmp.Or(os.Getenv("ARDUINO_APP_CLI__PLATFORM_VERSION_CONSTRAINT"), "<1.0.0")

	edgeImpulseAPIURL := os.Getenv("EDGE_IMPULSE_API_URL")
	if edgeImpulseAPIURL == "" {
		edgeImpulseAPIURL = "https://studio.edgeimpulse.com/v1"
	}

	parsedEdgeImpulseURL, err := url.Parse(edgeImpulseAPIURL)
	if err != nil {
		return Configuration{}, fmt.Errorf("invalid EDGE_IMPULSE_API_URL: %w", err)
	}

	constraint, err := semver.ParseConstraint(constraintStr)
	if err != nil {
		return Configuration{}, fmt.Errorf("invalid version constraint: %w", err)
	}
	slog.Debug("Using update version constraint", slog.String("constraint", constraintStr))

	c := Configuration{
		appsDir:                          appsDir,
		dataDir:                          dataDir,
		routerSocketPath:                 routerSocket,
		customModelsDir:                  customModelsDir,
		PythonImage:                      pythonImage,
		UsedPythonImageTag:               usedPythonImageTag,
		RunnerVersion:                    RunnerVersion,
		AllowRoot:                        allowRoot,
		LibrariesAPIURL:                  parsedLibrariesURL,
		EdgeImpulseAPIURL:                parsedEdgeImpulseURL,
		ArduinoPlatformVersionConstraint: constraint,
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

func (c *Configuration) CustomModelsDir() *paths.Path {
	return c.customModelsDir
}

func getPythonImageAndTag() (string, string) {
	registryBase := os.Getenv("DOCKER_REGISTRY_BASE")
	if registryBase == "" {
		registryBase = "ghcr.io/arduino/"
	}

	// Python image: image name (repository) and optionally a tag.
	pythonImageAndTag := os.Getenv("DOCKER_PYTHON_BASE_IMAGE")
	if pythonImageAndTag == "" {
		pythonImageAndTag = fmt.Sprintf("app-bricks/python-apps-base:%s", RunnerVersion)
	}
	pythonImage := path.Join(registryBase, pythonImageAndTag)
	var usedPythonImageTag string
	if idx := strings.LastIndex(pythonImage, ":"); idx != -1 {
		usedPythonImageTag = pythonImage[idx+1:]
	}
	return pythonImage, usedPythonImageTag
}
