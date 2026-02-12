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
	"strings"

	"github.com/arduino/go-paths-helper"
	semver "go.bug.st/relaxed-semver"
)

// runnerVersion do not edit, this is generate with `task generate:assets`
var RunnerVersion = "0.7.2"

type Configuration struct {
	appsDir                          *paths.Path
	dataDir                          *paths.Path
	routerSocketPath                 *paths.Path
	customModelsDir                  *paths.Path
	pythonImage                      string
	usedPythonImageTag               string
	runnerVersion                    string
	allowRoot                        bool
	librariesAPIURL                  *url.URL
	edgeImpulseAPIURL                *url.URL
	arduinoPlatformVersionConstraint semver.Constraint

	// New fields from issue
	compileSketchCores      int
	appStartParallelization int
	allowMultiAppStart      bool
}

// rawConfig is used internally to parse values from Env, TOML or JSON.
// Tags define the mapping and default values.
type rawConfig struct {
	AppsDir               string `env:"ARDUINO_APP_CLI__APPS_DIR" toml:"apps_dir"`
	DataDir               string `env:"ARDUINO_APP_CLI__DATA_DIR" toml:"data_dir"`
	RouterSocketPath      string `env:"ARDUINO_ROUTER_SOCKET" toml:"router_socket_path"`
	CustomModelsDir       string `env:"ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR" toml:"custom_models_dir"`
	DockerRegistryBase    string `env:"DOCKER_REGISTRY_BASE" toml:"docker_registry_base"`
	DockerPythonBaseImage string `env:"DOCKER_PYTHON_BASE_IMAGE" toml:"docker_python_base_image"`
	AllowRoot             bool   `env:"ARDUINO_APP_CLI__ALLOW_ROOT" toml:"allow_root"`
	LibrariesAPIURL       string `env:"LIBRARIES_API_URL" toml:"libraries_api_url"`
	EdgeImpulseAPIURL     string `env:"EDGE_IMPULSE_API_URL" toml:"edge_impulse_api_url"`
	VersionConstraint     string `env:"ARDUINO_APP_CLI__PLATFORM_VERSION_CONSTRAINT" toml:"version_constraint"`

	// New fields
	// todo check how the works and be sure about behaviors, defaults, etc...
	CompileSketchCores      int  `env:"ARDUINO_APP_CLI__COMPILE_SKETCH_CORES" toml:"compile_sketch_cores"`
	AppStartParallelization int  `env:"ARDUINO_APP_CLI__APP_START_PARALLELIZATION" toml:"app_start_parallelization"`
	AllowMultiAppStart      bool `env:"ARDUINO_APP_CLI__ALLOW_MULTI_APP_START" toml:"allow_multi_app_start"`
}

const DefaultConfigPath = "/home/arduino/Desktop/config.toml"

// Load initializes the configuration from environment variables and an optional config file.
func Load() (Configuration, error) {
	var raw rawConfig

	parser := New().WithParser(EnvParser())

	if DefaultConfigPath != "" {
		// Check if file exists before adding the parser to avoid panic/errors in the library
		if _, err := os.Stat(DefaultConfigPath); err == nil {
			parser.WithParser(TomlParser(DefaultConfigPath))
		}
	}

	if err := parser.Parse(&raw); err != nil {
		return Configuration{}, fmt.Errorf("failed to parse configuration: %w", err)
	}

	return mapRawToConfig(raw)
}

func mapRawToConfig(raw rawConfig) (Configuration, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Configuration{}, fmt.Errorf("could not get home dir: %w", err)
	}

	appsDir := paths.New(raw.AppsDir)
	if appsDir == nil {
		appsDir = paths.New(home).Join("ArduinoApps")
	}

	if !appsDir.IsAbs() {
		wd, err := paths.Getwd()
		if err != nil {
			return Configuration{}, err
		}
		appsDir = wd.JoinPath(appsDir)
	}

	// 2. Resolve DataDir
	dataDir := paths.New(raw.DataDir)
	if dataDir == nil {
		dataDir = paths.New("/var/lib/arduino-app-cli")
	}
	// todo check if we need to resolve it. the original code doesnt do that
	if !dataDir.IsAbs() {
		wd, _ := paths.Getwd()
		dataDir = wd.JoinPath(dataDir)
	}

	routerSocket := paths.New(raw.RouterSocketPath)
	if routerSocket == nil || routerSocket.NotExist() {
		routerSocket = paths.New("/var/run/arduino-router.sock")
	}
	// 4. Resolve CustomModelsDir
	customModelsDir := paths.New(raw.CustomModelsDir)
	if customModelsDir == nil {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return Configuration{}, err
		}
		customModelsDir = paths.New(homeDir).Join(".arduino-bricks").Join("models")
	}
	if customModelsDir.NotExist() {
		if err := customModelsDir.MkdirAll(); err != nil {
			slog.Warn("failed create custom model directory", "error", err)
		}
	}

	// 4. Docker Image Logic
	pythonImage, usedPythonImageTag := getPythonImageAndTag(raw.DockerRegistryBase, raw.DockerPythonBaseImage)
	slog.Debug("Using pythonImage", slog.String("image", pythonImage))

	// 5. Parsing complex types (URL, Semver)
	librariesAPIURL := raw.LibrariesAPIURL
	if librariesAPIURL == "" {
		librariesAPIURL = "https://api2.arduino.cc/libraries/v1/libraries"
	}
	parsedLibrariesURL, err := url.Parse(librariesAPIURL)
	if err != nil {
		return Configuration{}, fmt.Errorf("invalid LIBRARIES_API_URL: %w", err)
	}

	edgeImpulseAPIURL := raw.EdgeImpulseAPIURL
	if edgeImpulseAPIURL == "" {
		edgeImpulseAPIURL = "https://studio.edgeimpulse.com/v1"
	}

	parsedEdgeImpulseURL, err := url.Parse(edgeImpulseAPIURL)
	if err != nil {
		return Configuration{}, fmt.Errorf("invalid EDGE_IMPULSE_API_URL: %w", err)
	}

	constraintStr := raw.VersionConstraint
	if constraintStr == "" {
		constraintStr = "<1.0.0"
	}
	constraint, err := semver.ParseConstraint(constraintStr)
	if err != nil {
		return Configuration{}, fmt.Errorf("invalid version constraint: %w", err)
	}
	// todo check how this behaviors
	CompileSketchCores := raw.CompileSketchCores
	if CompileSketchCores == 0 {
		CompileSketchCores = 1
	}
	// todo check how this behaviors
	AppStartParallelization := raw.AppStartParallelization
	if AppStartParallelization == 0 {
		AppStartParallelization = 1
	}

	slog.Debug("Using update version constraint", slog.String("constraint", constraintStr))
	c := Configuration{
		appsDir:                          appsDir,
		dataDir:                          dataDir,
		routerSocketPath:                 routerSocket,
		customModelsDir:                  customModelsDir,
		pythonImage:                      pythonImage,
		usedPythonImageTag:               usedPythonImageTag,
		runnerVersion:                    RunnerVersion,
		allowRoot:                        raw.AllowRoot,
		librariesAPIURL:                  parsedLibrariesURL,
		edgeImpulseAPIURL:                parsedEdgeImpulseURL,
		arduinoPlatformVersionConstraint: constraint,
		compileSketchCores:               CompileSketchCores,
		appStartParallelization:          AppStartParallelization,
		// todo check how this behaviors
		allowMultiAppStart: raw.AllowMultiAppStart,
	}

	if err := c.init(); err != nil {
		return Configuration{}, err
	}

	dumpPath := "/home/arduino/Desktop/config_dump_new.txt"
	dumpContent := fmt.Sprintf(`--- NEW CONFIG DUMP ---
AppsDir:           %s
DataDir:           %s
RouterSocketPath:  %s
CustomModelsDir:   %s
PythonImage:       %s
UsedPythonImageTag:%s
AllowRoot:         %t
LibrariesURL:      %s
Constraint:        %s
CompileCores:      %d
Parallelization:   %d
MultiAppStart:     %t
----------------------------
`,
		c.appsDir, c.dataDir, c.routerSocketPath, c.customModelsDir,
		c.pythonImage, c.usedPythonImageTag, c.allowRoot,
		c.librariesAPIURL, constraintStr,
		c.compileSketchCores, c.appStartParallelization, c.allowMultiAppStart)

	_ = os.WriteFile(dumpPath, []byte(dumpContent), 0600)

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
	if c.customModelsDir.NotExist() {
		if err := c.customModelsDir.MkdirAll(); err != nil {
			slog.Warn("failed create custom model directory", "error", err)
		}
	}
	return nil
}

// --- Getters ---

func (c *Configuration) AppsDir() *paths.Path { return c.appsDir }

func (c *Configuration) DataDir() *paths.Path { return c.dataDir }

func (c *Configuration) ExamplesDir() *paths.Path { return c.dataDir.Join("examples") }

func (c *Configuration) RouterSocketPath() *paths.Path { return c.routerSocketPath }

func (c *Configuration) AssetsDir() *paths.Path { return c.dataDir.Join("assets") }

func (c *Configuration) CustomModelsDir() *paths.Path { return c.customModelsDir }

func (c *Configuration) PythonImage() string { return c.pythonImage }

func (c *Configuration) UsedPythonImageTag() string { return c.usedPythonImageTag }

func (c *Configuration) RunnerVersion() string { return c.runnerVersion }

func (c *Configuration) AllowRoot() bool { return c.allowRoot }

func (c *Configuration) LibrariesAPIURL() *url.URL   { return c.librariesAPIURL }
func (c *Configuration) EdgeImpulseAPIURL() *url.URL { return c.edgeImpulseAPIURL }

func (c *Configuration) ArduinoPlatformVersionConstraint() semver.Constraint {
	return c.arduinoPlatformVersionConstraint
}

// New fields
func (c *Configuration) CompileSketchCores() int { return c.compileSketchCores }

func (c *Configuration) AppStartParallelization() int { return c.appStartParallelization }

func (c *Configuration) AllowMultiAppStart() bool { return c.allowMultiAppStart }

func getPythonImageAndTag(registryRaw, baseImageRaw string) (string, string) {
	registryBase := registryRaw
	if registryBase == "" {
		registryBase = "ghcr.io/arduino/"
	}

	// Python image: image name (repository) and optionally a tag.
	pythonImageAndTag := baseImageRaw
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
