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

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/user"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/arduino/go-paths-helper"
	"github.com/containerd/errdefs"
	"github.com/docker/cli/cli/command"
	"github.com/docker/docker/api/types/container"
	yaml "github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/helpers"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/peripherals"
	"github.com/arduino/arduino-app-cli/internal/store"
)

type volume struct {
	Type   string `yaml:"type"`
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

type dependsOnCondition struct {
	Condition string `yaml:"condition"`
}

type logging struct {
	Driver  string            `yaml:"driver"`
	Options map[string]string `yaml:"options,omitempty"`
}

type service struct {
	Image       string                        `yaml:"image"`
	DependsOn   map[string]dependsOnCondition `yaml:"depends_on,omitempty"`
	Volumes     []volume                      `yaml:"volumes"`
	Devices     []string                      `yaml:"devices"`
	Ports       []string                      `yaml:"ports"`
	User        string                        `yaml:"user"`
	GroupAdd    []uint32                      `yaml:"group_add"`
	Entrypoint  string                        `yaml:"entrypoint"`
	ExtraHosts  []string                      `yaml:"extra_hosts,omitempty"`
	Labels      map[string]string             `yaml:"labels,omitempty"`
	Environment map[string]string             `yaml:"environment,omitempty"`
	Logging     *logging                      `yaml:"logging,omitempty"`
}

type Provision struct {
	docker      command.Cli
	pythonImage string
}

func isDevelopmentMode(cfg config.Configuration) bool {
	return cfg.RunnerVersion != cfg.UsedPythonImageTag
}

func NewProvision(
	docker command.Cli,
	cfg config.Configuration,
) (*Provision, error) {
	provision := &Provision{
		docker:      docker,
		pythonImage: cfg.PythonImage,
	}

	dynamicProvisionDir := cfg.AssetsDir().Join(cfg.UsedPythonImageTag)

	// In development mode we want to make sure everything is fresh.
	if isDevelopmentMode(cfg) {
		_ = dynamicProvisionDir.RemoveAll()
	}

	if dynamicProvisionDir.Exist() {
		return provision, nil
	}

	tmpProvisionDir, err := cfg.AssetsDir().MkTempDir("dynamic-provisioning")
	if err != nil {
		return nil, fmt.Errorf("failed to perform creation of dynamic provisioning dir: %w", err)
	}
	if err := provision.init(tmpProvisionDir.String()); err != nil {
		return nil, fmt.Errorf("failed to perform dynamic provisioning: %w", err)
	}
	if err := tmpProvisionDir.Rename(dynamicProvisionDir); err != nil {
		return nil, fmt.Errorf("failed to rename tmp provisioning folder: %w", err)
	}

	return provision, nil
}

func (p *Provision) App(
	ctx context.Context,
	bricksIndex *bricksindex.BricksIndex,
	arduinoApp *app.ArduinoApp,
	cfg config.Configuration,
	mapped_env map[string]string,
	staticStore *store.StaticStore,
	devices peripherals.AvailableDevices,
) error {
	if arduinoApp == nil {
		return fmt.Errorf("provisioning failed: arduinoApp is nil")
	}

	if arduinoApp.ProvisioningStateDir().NotExist() {
		if err := arduinoApp.ProvisioningStateDir().MkdirAll(); err != nil {
			return fmt.Errorf("provisioning failed: unable to create .cache")
		}
	}

	return generateMainComposeFile(arduinoApp, bricksIndex, p.pythonImage, cfg, mapped_env, staticStore, devices)
}

func (p *Provision) init(
	srcPath string,
) error {
	containerCfg := &container.Config{
		Image: p.pythonImage,
		User:  getCurrentUser(),
		Entrypoint: []string{
			"/bin/bash",
			"-c",
			fmt.Sprintf("%s && %s",
				"arduino-bricks-list-modules -o /app/bricks-list.yaml -m /app/models-list.yaml",
				"arduino-bricks-list-modules --provision-compose -o /app",
			),
		},
	}
	containerHostCfg := &container.HostConfig{
		Binds:      []string{srcPath + ":/app"},
		AutoRemove: true,
	}
	resp, err := p.docker.Client().ContainerCreate(context.Background(), containerCfg, containerHostCfg, nil, nil, "")
	if err != nil {
		if errors.Is(err, errdefs.ErrNotFound) {
			if err := pullBasePythonContainer(context.Background(), p.pythonImage); err != nil {
				return fmt.Errorf("provisioning failed to pull base image: %w", err)
			}
			// Now that we have pulled the container we recreate it
			resp, err = p.docker.Client().ContainerCreate(context.Background(), containerCfg, containerHostCfg, nil, nil, "")
		}
		if err != nil {
			return fmt.Errorf("provisiong failed to create container: %w", err)
		}
	}

	slog.Debug("provisioning container created", slog.String("container_id", resp.ID))

	waitCh, errCh := p.docker.Client().ContainerWait(context.Background(), resp.ID, container.WaitConditionNextExit)
	if err := p.docker.Client().ContainerStart(context.Background(), resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("provisioning failed to start container: %w", err)
	}
	slog.Debug("provisioning container started", slog.String("container_id", resp.ID))

	select {
	case result := <-waitCh:
		if result.Error != nil {
			return fmt.Errorf("provisioning failed: %v", result.Error.Message)
		}
	case err := <-errCh:
		return fmt.Errorf("provisioning failed: %w", err)
	}
	return nil
}

func pullBasePythonContainer(ctx context.Context, pythonImage string) error {
	process, err := paths.NewProcess(nil, "docker", "pull", pythonImage)
	if err != nil {
		return err
	}
	process.RedirectStdoutTo(NewCallbackWriter(func(line string) {
		slog.Debug("Pulling container", slog.String("image", pythonImage), slog.String("line", line))
	}))
	process.RedirectStderrTo(NewCallbackWriter(func(line string) {
		slog.Error("Error pulling container", slog.String("image", pythonImage), slog.String("line", line))
	}))
	return process.RunWithinContext(ctx)
}

const (
	DockerAppLabel     = "cc.arduino.app"
	DockerAppMainLabel = "cc.arduino.app.main"
	DockerAppPathLabel = "cc.arduino.app.path"
)

func generateMainComposeFile(
	app *app.ArduinoApp,
	bricksIndex *bricksindex.BricksIndex,
	pythonImage string,
	cfg config.Configuration,
	envs helpers.EnvVars,
	staticStore *store.StaticStore,
	devices peripherals.AvailableDevices,
) error {
	slog.Debug("Generating main compose file for the App")

	ports := make(map[string]struct{}, len(app.Descriptor.Ports))
	for _, p := range app.Descriptor.Ports {
		ports[fmt.Sprintf("%d:%d", p, p)] = struct{}{}
	}

	var composeFiles paths.PathList
	services := make([]serviceInfo, 0, len(app.Descriptor.Bricks))
	for _, brick := range app.Descriptor.Bricks {
		idxBrick, found := bricksIndex.FindBrickByID(brick.ID)
		slog.Debug("Processing brick", slog.String("brick_id", brick.ID), slog.Bool("found", found))
		if !found {
			continue
		}

		// 1. Retrieve ports that we have to expose defined in the brick
		for _, p := range idxBrick.Ports {
			ports[fmt.Sprintf("%s:%s", p, p)] = struct{}{}
		}

		// The following code is needed only if the brick requires a container.
		// In case it doesn't we just skip to the next one.
		if !idxBrick.RequireContainer {
			continue
		}

		// 3. Retrieve the brick_compose.yaml file.
		composeFilePath, err := staticStore.GetBrickComposeFilePathFromID(brick.ID)
		if err != nil {
			slog.Error("brick compose id not valid", slog.String("error", err.Error()), slog.String("brick_id", brick.ID))
			continue
		}

		// 4. Retrieve the compose services names.
		svcs, err := extractServicesFromComposeFile(composeFilePath)
		if err != nil {
			slog.Error("loading brick_compose", slog.String("brick_id", brick.ID), slog.String("path", composeFilePath.String()), slog.Any("error", err))
			continue
		}

		// 5. Retrieve the required devices that we have to mount
		slog.Debug("Brick config", slog.Bool("mount_devices_into_container", idxBrick.MountDevicesIntoContainer), slog.Any("ports", ports), slog.Any("required_devices", idxBrick.RequiredDevices))
		if idxBrick.MountDevicesIntoContainer {
			for i := range svcs {
				svcs[i].requireDevices = true
			}
		}

		composeFiles.Add(composeFilePath)
		services = append(services, svcs...)
	}

	if len(app.Descriptor.RequiredDevices) > 0 { // nolint:staticcheck
		slog.Warn("The 'required_devices' field is deprecated. Please move requirements to the specific 'bricks' section.")
	}

	// Create a single docker-mainCompose that includes all the required services
	mainComposeFile := app.AppComposeFilePath()
	// If required, create an override compose file for devices
	overrideComposeFile := app.AppComposeOverrideFilePath()

	type mainService struct {
		Main service `yaml:"main"`
	}
	var mainAppCompose struct {
		Name     string       `yaml:"name"`
		Include  []string     `yaml:"include,omitempty"`
		Services *mainService `yaml:"services,omitempty"`
	}
	// Merge compose
	composeProjectName, err := getAppComposeProjectNameFromApp(*app, cfg)
	if err != nil {
		return err
	}
	mainAppCompose.Name = composeProjectName
	mainAppCompose.Include = composeFiles.AsStrings()

	volumes := []volume{
		{
			Type:   "bind",
			Source: app.FullPath.String(),
			Target: "/app",
		},
	}
	slog.Debug("Adding UNIX socket", slog.Any("sock", cfg.RouterSocketPath().String()), slog.Bool("exists", cfg.RouterSocketPath().Exist()))
	if cfg.RouterSocketPath().Exist() {
		volumes = append(volumes, volume{
			Type:   "bind",
			Source: cfg.RouterSocketPath().String(),
			Target: "/var/run/arduino-router.sock",
		})
	}

	// provide additional devices to the container
	if devices.HasVideoDevice {
		// If we are adding video devices, mount also /dev/v4l if it exists to allow access to by-id/path links
		if paths.New("/dev/v4l").Exist() {
			volumes = append(volumes, volume{
				Type:   "bind",
				Source: "/dev/v4l",
				Target: "/dev/v4l",
			})
		}
	}
	if devices.HasSoundDevice {
		// If we are adding sound devices, mount also /dev/snd/by-id if it exists to allow access to by-id links
		if paths.New("/dev/snd/by-id").Exist() {
			volumes = append(volumes, volume{
				Type:   "bind",
				Source: "/dev/snd/by-id",
				Target: "/dev/snd/by-id",
			})
		}
	}

	volumes = addLedControl(volumes)
	groups := lookupGroups("video", "audio", "render", "dialout")

	// Define depends_on conditions
	// Services with healthcheck will be started only when healthy
	// Services without healthcheck will be started as soon as the container is started
	dependsOn := make(map[string]dependsOnCondition, len(services))
	for _, s := range services {
		if s.hasHealthcheck {
			dependsOn[s.name] = dependsOnCondition{
				Condition: "service_healthy",
			}
		} else {
			dependsOn[s.name] = dependsOnCondition{
				Condition: "service_started",
			}
		}
	}

	mainAppCompose.Services = &mainService{
		Main: service{
			Image:      pythonImage,
			Volumes:    volumes,
			Ports:      slices.Collect(maps.Keys(ports)),
			Devices:    devices.DevicePaths,
			Entrypoint: "/run.sh",
			DependsOn:  dependsOn,
			User:       getCurrentUser(),
			GroupAdd:   append(groups, lookupGroups("gpiod")...),
			ExtraHosts: []string{"msgpack-rpc-router:host-gateway"},
			Labels: map[string]string{
				DockerAppLabel:     "true",
				DockerAppMainLabel: "true",
				DockerAppPathLabel: app.FullPath.String(),
			},
			Environment: envs,
			Logging: &logging{
				Driver: "json-file",
				Options: map[string]string{
					"max-size": "5m",
					"max-file": "2",
				},
			},
		},
	}

	// Write the main compose file
	data, err := yaml.Marshal(mainAppCompose)
	if err != nil {
		return err
	}
	if err := mainComposeFile.WriteFile(data); err != nil {
		return err
	}

	// If there are services that require devices, we need to generate an override compose file
	// Write additional file to override devices section in included compose files
	if err := generateServicesOverrideFile(app, services, devices.DevicePaths, getCurrentUser(), groups, overrideComposeFile, envs); err != nil {
		return err
	}

	// Pre-provision containers required paths, if they do not exist.
	// This is required to preserve the host directory access rights for arduino user.
	// Otherwise, paths created by the container will have root:root ownership
	for _, additionalComposeFile := range composeFiles {
		composeFilePath := additionalComposeFile.String()
		slog.Debug("Pre-provisioning volumes from compose file", slog.String("compose_file", composeFilePath))

		volumes, err := extractVolumesFromComposeFile(composeFilePath)
		if err != nil {
			slog.Warn("Failed to extract volumes from compose file", slog.String("compose_file", composeFilePath), slog.Any("error", err))
			continue
		}
		provisionComposeVolumes(composeFilePath, volumes, app, envs)
	}

	// Done!
	return nil
}

// Resolve supplementary group IDs on the host dynamically
// before assigning them to the container, as numeric GIDs
// could differ between host and container environments.
func lookupGroups(groupNames ...string) []uint32 {
	resolvedGids := make([]uint32, 0, len(groupNames))

	for _, name := range groupNames {
		g, err := user.LookupGroup(name)
		if err != nil {
			slog.Warn("group not found on host; skipping", "group", name)
			continue
		}
		gid, err := strconv.ParseUint(g.Gid, 10, 32)
		if err != nil {
			slog.Warn("failed to parse GID; skipping", "group", name)
			continue
		}
		resolvedGids = append(resolvedGids, uint32(gid))
	}
	return resolvedGids
}

type serviceInfo struct {
	name           string
	hasHealthcheck bool
	user           *string
	requireDevices bool
}

func extractServicesFromComposeFile(composeFile *paths.Path) ([]serviceInfo, error) {
	content, err := os.ReadFile(composeFile.String())
	if err != nil {
		return nil, err
	}

	type serviceMin struct {
		Image       string  `yaml:"image"`
		User        *string `yaml:"user,omitempty"`
		Healthcheck struct {
			Test []string `yaml:"test"`
		} `yaml:"healthcheck,omitempty"`
	}
	type composeServices struct {
		Services map[string]serviceMin `yaml:"services"`
	}
	var index composeServices
	if err := yaml.Unmarshal(content, &index); err != nil {
		return nil, err
	}
	services := make([]serviceInfo, 0, len(index.Services))
	for svc, svcDef := range index.Services {
		hasHealthcheck := len(svcDef.Healthcheck.Test) > 0
		services = append(services, serviceInfo{
			name:           svc,
			hasHealthcheck: hasHealthcheck,
			user:           svcDef.User,
		})
	}
	return services, nil
}

func generateServicesOverrideFile(arduinoApp *app.ArduinoApp, services []serviceInfo, devices []string, user string, groups []uint32, overrideComposeFile *paths.Path, envs helpers.EnvVars) error {
	if overrideComposeFile.Exist() {
		if err := overrideComposeFile.Remove(); err != nil {
			return fmt.Errorf("failed to remove existing override compose file: %w", err)
		}
	}

	if len(services) == 0 {
		slog.Debug("No services to override, skipping override compose file generation")
		return nil
	}

	type serviceOverride struct {
		User        *string           `yaml:"user,omitempty"`
		Devices     *[]string         `yaml:"devices,omitempty"`
		GroupAdd    *[]uint32         `yaml:"group_add,omitempty"`
		Labels      map[string]string `yaml:"labels,omitempty"`
		Environment map[string]string `yaml:"environment,omitempty"`
	}
	var overrideCompose struct {
		Services map[string]serviceOverride `yaml:"services,omitempty"`
	}
	overrideCompose.Services = make(map[string]serviceOverride, len(services))
	for _, svc := range services {
		override := serviceOverride{
			Labels: map[string]string{
				DockerAppLabel:     "true",
				DockerAppPathLabel: arduinoApp.FullPath.String(),
			},
			GroupAdd: &groups,
		}
		// If service defines a user, do not override it
		if svc.user == nil {
			override.User = &user
		}
		if svc.requireDevices {
			override.Devices = &devices
		}
		override.Environment = envs
		overrideCompose.Services[svc.name] = override
	}
	writeOverrideCompose := func() error {
		data, err := yaml.Marshal(overrideCompose)
		if err != nil {
			return err
		}
		if err := overrideComposeFile.WriteFile(data); err != nil {
			return err
		}
		return nil
	}
	if e := writeOverrideCompose(); e != nil {
		return e
	}
	return nil
}

var (
	// Regular expression to split on the first colon that is not followed by a hyphen
	volumeColonSplitRE     = regexp.MustCompile(`:[^-]`)
	volumeAppHomeReplaceRE = regexp.MustCompile(`\$\{APP_HOME(:-\.)?\}`)
	volumePathReplaceRE    = regexp.MustCompile(`\$\{([A-Z_-]+)(:-)?((?:\$\{[A-Z_-]+\}|[\/a-zA-Z0-9._-])*)?\}`)
)

// provisionComposeVolumes ensure we create the parent folder with the correct owner.
// By default docker if it doesn't find the folder, it will create it as root.
// We do not want that, to make sure to have it as `arduino:arduino` we have
// to manually parse the volumes, and make sure to create the target dirs ourself.
func provisionComposeVolumes(additionalComposeFile string, volumes []string, app *app.ArduinoApp, mapped_env map[string]string) {
	if len(volumes) == 0 {
		slog.Debug("No volumes to provision from compose file", slog.String("compose_file", additionalComposeFile))
		return
	}

	slog.Debug("Extracted volumes from compose file", slog.String("compose_file", additionalComposeFile), slog.Any("volumes", volumes))
	for _, volume := range volumes {
		volume = replaceDockerMacros(volume, app, mapped_env, additionalComposeFile)
		hostDirectory := paths.New(volume)
		if strings.Contains(volume, ":") {
			volumes := volumeColonSplitRE.Split(volume, -1)
			hostDirectory = paths.New(volumes[0])
		}
		if !hostDirectory.Exist() {
			if err := hostDirectory.MkdirAll(); err != nil {
				slog.Warn("Failed to create host directory for compose file", slog.String("compose_file", additionalComposeFile), slog.String("host_directory", hostDirectory.String()), slog.Any("error", err))
			} else {
				slog.Debug("Pre-provisioning host directory for compose file", slog.String("compose_file", additionalComposeFile), slog.String("host_directory", hostDirectory.String()))
			}
		}
	}
}

func replaceDockerMacros(volume string, app *app.ArduinoApp, mapped_env map[string]string, additionalComposeFile string) string {
	// Replace ${APP_HOME} with the actual app path
	volume = volumeAppHomeReplaceRE.ReplaceAllString(volume, app.FullPath.String())
	// Replace host volume directory with the actual path
	if volumePathReplaceRE.MatchString(volume) {
		groups := volumePathReplaceRE.FindStringSubmatch(volume)
		// idx 0 is the full match, idx 1 is the variable name, idx 2 is the optional `:-` and idx 3 is the default value
		switch len(groups) {
		case 2:
			// Check if the environment variable is set
			if value, ok := mapped_env[groups[1]]; ok {
				volume = volumePathReplaceRE.ReplaceAllString(volume, value)
			} else {
				slog.Warn("Environment variable not found for volume replacement", slog.String("variable", groups[1]), slog.String("compose_file", additionalComposeFile))
			}
		case 4:
			// If the variable is not set, use the default value
			if value, ok := mapped_env[groups[1]]; ok {
				volume = volumePathReplaceRE.ReplaceAllString(volume, value)
			} else {
				// Try to resolve with mapped environent variables as well
				resolved := os.Expand(groups[3], func(key string) string {
					if value, ok := mapped_env[key]; ok {
						return value
					}
					return os.Getenv(key)
				})
				volume = volumePathReplaceRE.ReplaceAllString(volume, resolved)
			}
		default:
			slog.Warn("Unexpected format for volume replacement", slog.String("volume", volume), slog.String("compose_file", additionalComposeFile))
		}
	}
	return volume
}

func extractVolumesFromComposeFile(additionalComposeFile string) ([]string, error) {
	content, err := os.ReadFile(additionalComposeFile)
	if err != nil {
		slog.Error("Failed to read compose file", slog.String("compose_file", additionalComposeFile), slog.Any("error", err))
		return nil, err
	}
	// Try with string syntax first
	type composeServices[T any] struct {
		Services map[string]struct {
			Volumes []T `yaml:"volumes"`
		} `yaml:"services"`
	}
	var index composeServices[string]
	if err := yaml.Unmarshal(content, &index); err != nil {
		var index composeServices[volume]
		if err := yaml.Unmarshal(content, &index); err != nil {
			return nil, fmt.Errorf("failed to unmarshal compose file %s: %w", additionalComposeFile, err)
		}
		volumes := make([]string, 0, len(index.Services))
		for _, svc := range index.Services {
			for _, v := range svc.Volumes {
				if v.Type == "bind" {
					volumes = append(volumes, v.Source)
				} else {
					volumes = append(volumes, v.Target)
				}
			}
		}
		return volumes, nil
	}

	volumes := make([]string, 0, len(index.Services))
	for _, svc := range index.Services {
		volumes = append(volumes, svc.Volumes...)
	}
	return volumes, nil
}
