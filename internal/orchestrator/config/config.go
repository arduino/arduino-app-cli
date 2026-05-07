// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

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
var RunnerVersion = "0.10.0rc2"

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
	CgroupRules                      []string
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

	cgroupDrivers := map[string]string{
		"drm":         "DRM",
		"dma_heap":    "DMA Heap",
		"media":       "Media",
		"video4linux": "video4linux",
		"alsa":        "ALSA",
	}

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
		CgroupRules:                      buildCgroupRules(cgroupDrivers),
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

func buildCgroupRules(drivers map[string]string) []string {
	var rules []string

	for driver, label := range drivers {
		major, err := resolveMajorNumber(driver)
		if err != nil {
			slog.Warn("could not resolve major number, skipping cgroup rule",
				slog.String("driver", driver),
				slog.String("label", label),
				slog.Any("error", err),
			)
			continue
		}
		rules = append(rules, fmt.Sprintf("c %d:* rmw", major))
	}

	return rules
}

func resolveMajorNumber(driverName string) (int, error) {
	content, err := os.ReadFile("/proc/devices")
	if err != nil {
		return 0, fmt.Errorf("failed to read /proc/devices: %w", err)
	}
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == driverName {
			major, err := strconv.Atoi(fields[0])
			if err != nil {
				return 0, fmt.Errorf("failed to parse major for %s: %w", driverName, err)
			}
			return major, nil
		}
	}
	return 0, fmt.Errorf("driver %q not found in /proc/devices", driverName)
}
