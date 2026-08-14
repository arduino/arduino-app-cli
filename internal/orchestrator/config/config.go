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

	"github.com/arduino/arduino-app-cli/internal/platform"
)

// runnerVersion do not edit, this is generate with `task bump:runner-version`
var RunnerVersion = "0.12.0"

type Configuration struct {
	appsDir                          *paths.Path
	dataDir                          *paths.Path
	requiredRuntimes                 []RequiredRuntime
	customModelsDir                  *paths.Path
	modelsDir                        *paths.Path
	assetDir                         *paths.Path
	dockerRegistryBase               string
	usedPythonImageTag               string
	PythonImage                      string
	RunnerVersion                    string
	AllowRoot                        bool
	LibrariesAPIURL                  *url.URL
	EdgeImpulseAPIURL                *url.URL
	ArduinoPlatformVersionConstraint semver.Constraint
}

// RequiredRuntime is a host unit whose socket is bind-mounted into app
// containers, together with the supplementary group needed to access it.
type RequiredRuntime struct {
	Unit  string
	Group string
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

	// Required host units bind-mounted as /run/<unit> into app containers.
	// Each entry is `<unit>[:<group>]`, where the optional group is the host
	// group required to access the unit socket.
	requiredRuntimesEnv, ok := os.LookupEnv("ARDUINO_APP_CLI__REQUIRED_RUNTIMES")
	if !ok {
		requiredRuntimesEnv = "arduino-router:arduino-router,arduino-cloud-connector"
	}
	var requiredRuntimes []RequiredRuntime
	for entry := range strings.SplitSeq(requiredRuntimesEnv, ",") {
		unit, group, _ := strings.Cut(strings.TrimSpace(entry), ":")
		if unit = strings.TrimSpace(unit); unit != "" {
			requiredRuntimes = append(requiredRuntimes, RequiredRuntime{
				Unit:  unit,
				Group: strings.TrimSpace(group),
			})
		}
	}

	// Directory where all AI models are installed.
	modelsDir := paths.New(os.Getenv("MODELS_PATH"))
	if modelsDir == nil {
		modelsDir = dataDir.Join("models")
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

	registryBase := getDockerRegistryBase()
	pythonImage, usedPythonImageTag := getPythonImageAndTag(registryBase)
	slog.Debug("Using pythonImage", slog.String("image", pythonImage))

	assetsDir := dataDir.Join("assets").Join(usedPythonImageTag)

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
		requiredRuntimes:                 requiredRuntimes,
		customModelsDir:                  customModelsDir,
		modelsDir:                        modelsDir,
		assetDir:                         assetsDir,
		dockerRegistryBase:               registryBase,
		PythonImage:                      pythonImage,
		usedPythonImageTag:               usedPythonImageTag,
		RunnerVersion:                    RunnerVersion,
		AllowRoot:                        allowRoot,
		LibrariesAPIURL:                  parsedLibrariesURL,
		EdgeImpulseAPIURL:                parsedEdgeImpulseURL,
		ArduinoPlatformVersionConstraint: constraint,
	}

	return c, nil
}

// EnsureFolders creates the folders required by arduino-app-cli.
//
// This must not be executed as root (e.g. under ALLOW_ROOT): the folders would
// be created root-owned and cause permission issues for the arduino user that
// runs the application. Callers should skip it when running as root.
func (c *Configuration) EnsureFolders() error {
	if err := c.AppsDir().MkdirAll(); err != nil {
		return err
	}
	if err := c.ModelsDir().MkdirAll(); err != nil {
		return err
	}
	if err := c.AssetDir().MkdirAll(); err != nil {
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

func (c *Configuration) ExamplesBaseDir() *paths.Path {
	return c.dataDir.Join("examples")
}

func (c *Configuration) ExamplesAdditionalDirs() paths.PathList {
	return paths.PathList{
		c.ExamplesBaseDir().Join("core-and-foundational"),
		c.ExamplesBaseDir().Join("bricks")}
}

func (c *Configuration) ExamplesDirs(platform platform.Platform) paths.PathList {
	boardExampleDir := c.ExamplesBaseDir().Join("inspirational").Join(fmt.Sprintf("platform_%s", platform.BoardName))
	if boardExampleDir.Exist() {
		return paths.PathList{boardExampleDir, c.ExamplesBaseDir().Join("inspirational").Join("common")}
	}
	return paths.PathList{c.ExamplesBaseDir().Join("inspirational").Join("common")}
}

type ResolvedRequiredRuntime struct {
	Path  *paths.Path
	Group string
}

// RequiredRuntimes returns the configured required units that are available on
// the host, each paired with the group needed to access its socket. The socket
// path is searched in order: /run/<unit>, /var/run/<unit>, /run/<unit>.sock,
// /var/run/<unit>.sock. The first existing entry per unit is returned.
func (c *Configuration) RequiredRuntimes() []ResolvedRequiredRuntime {
	var result []ResolvedRequiredRuntime
	seen := map[string]bool{}
	for _, runtime := range c.requiredRuntimes {
		candidates := []*paths.Path{
			paths.New("/run", runtime.Unit),
			paths.New("/var/run", runtime.Unit),
			paths.New("/run", runtime.Unit+".sock"),
			paths.New("/var/run", runtime.Unit+".sock"),
		}
		found := false
		for _, p := range candidates {
			if p.Exist() {
				if !seen[p.String()] {
					seen[p.String()] = true
					result = append(result, ResolvedRequiredRuntime{
						Path:  p,
						Group: runtime.Group,
					})
				}
				found = true
				break
			}
		}
		if !found {
			slog.Debug("required runtime not found on host", "runtime", runtime.Unit)
		}
	}
	return result
}

func (c *Configuration) AssetDir() *paths.Path {
	return c.assetDir
}

func (c *Configuration) MkTempAssetDir() (*paths.Path, error) {
	return c.assetDir.Parent().MkTempDir("dynamic-provisioning")
}

func (c *Configuration) CustomModelsDir() *paths.Path {
	return c.customModelsDir
}

func (c *Configuration) ModelsDir() *paths.Path {
	return c.modelsDir
}

func (c *Configuration) DockerRegistryBase() string {
	return c.dockerRegistryBase
}

func (c *Configuration) IsDevelopmentMode() bool {
	return c.RunnerVersion != c.usedPythonImageTag
}

func getDockerRegistryBase() string {
	registryBase := os.Getenv("DOCKER_REGISTRY_BASE")
	if registryBase == "" {
		registryBase = "ghcr.io/arduino/"
	}
	return registryBase
}

func getPythonImageAndTag(registryBase string) (string, string) {
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
