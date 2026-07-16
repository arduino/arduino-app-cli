// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/user"
	"slices"
	"strconv"
	"strings"

	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/containerd/errdefs"
	"github.com/docker/cli/cli/command"
	"github.com/docker/docker/api/types/container"
	yaml "github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/helpers"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

type volume struct {
	Type     string `yaml:"type"`
	Source   string `yaml:"source"`
	Target   string `yaml:"target"`
	ReadOnly bool   `yaml:"read_only,omitempty"`
}

type dependsOnCondition struct {
	Condition string `yaml:"condition"`
}

type logging struct {
	Driver  string            `yaml:"driver"`
	Options map[string]string `yaml:"options,omitempty"`
}

type service struct {
	Image             string                        `yaml:"image"`
	DependsOn         map[string]dependsOnCondition `yaml:"depends_on,omitempty"`
	Volumes           []volume                      `yaml:"volumes"`
	DeviceCgroupRules []string                      `yaml:"device_cgroup_rules,omitempty"`
	Ports             []string                      `yaml:"ports"`
	User              string                        `yaml:"user"`
	GroupAdd          []uint32                      `yaml:"group_add"`
	Entrypoint        string                        `yaml:"entrypoint"`
	ExtraHosts        []string                      `yaml:"extra_hosts,omitempty"`
	Labels            map[string]string             `yaml:"labels,omitempty"`
	Environment       map[string]string             `yaml:"environment,omitempty"`
	Logging           *logging                      `yaml:"logging,omitempty"`
}

type Provision struct {
	docker      command.Cli
	pythonImage string
}

func NewProvision(
	docker command.Cli,
	cfg config.Configuration,
) (*Provision, error) {
	provision := &Provision{
		docker:      docker,
		pythonImage: cfg.PythonImage,
	}

	dynamicProvisionDir := cfg.AssetDir()

	// In development mode we want to make sure everything is fresh.
	if cfg.IsDevelopmentMode() {
		_ = dynamicProvisionDir.RemoveAll()
	}

	if dynamicProvisionDir.Exist() {
		return provision, nil
	}

	tmpProvisionDir, err := cfg.MkTempAssetDir()
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
	bricksIndex *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
	arduinoApp *app.ArduinoApp,
	cfg config.Configuration,
	mapped_env map[string]string,
	platform platform.Platform,
) error {
	if arduinoApp == nil {
		return fmt.Errorf("provisioning failed: arduinoApp is nil")
	}

	if arduinoApp.ProvisioningStateDir().NotExist() {
		if err := arduinoApp.ProvisioningStateDir().MkdirAll(); err != nil {
			return fmt.Errorf("provisioning failed: unable to create .cache")
		}
	}

	bricksIndex = bricksIndex.WithAppBricks(arduinoApp.LocalBricks)

	return generateMainComposeFile(arduinoApp, bricksIndex, servicesIndex, p.pythonImage, cfg, mapped_env, platform)
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
	servicesIndex *servicesindex.ServicesIndex,
	pythonImage string,
	cfg config.Configuration,
	envs helpers.EnvVars,
	platform platform.Platform,
) error {
	slog.Debug("Generating main compose file for the App")

	ports := make(map[string]struct{}, len(app.Descriptor.Ports))
	for _, p := range app.Descriptor.Ports {
		ports[fmt.Sprintf("%d:%d", p, p)] = struct{}{}
	}

	brickServices := make(map[string]servicesindex.Service)
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

		// 2. Retrieve the required singleton services
		matchingServices, err := idxBrick.GetMatchingService(bricksindex.BrickInstance{
			Model: cmp.Or(brick.Model, idxBrick.ModelName),
		})
		if err != nil {
			return fmt.Errorf("failed to get required services for brick %s: %w", brick.ID, err)
		}
		for _, id := range matchingServices {
			service, found := servicesIndex.FindServiceByID(id)
			if !found {
				slog.Debug("service required by brick not found or not available for current board", slog.String("service_id", id), slog.String("brick_id", brick.ID))
				continue
			}
			brickServices[id] = *service
		}

		// 3. Retrieve the brick_compose.yaml file.
		composeFilePath, ok := idxBrick.GetComposeFile()
		if !ok {
			continue
		}

		// 4. Retrieve the compose services names.
		svcs, err := extractServicesFromComposeFile(composeFilePath)
		if err != nil {
			slog.Warn("loading brick_compose", slog.String("brick_id", brick.ID), slog.String("path", composeFilePath.String()), slog.Any("error", err))
			continue
		}

		if len(svcs) == 0 {
			continue
		}

		// 5. Retrieve the required devices that we have to mount
		slog.Debug("Brick config", slog.Bool("mount_devices_into_container", idxBrick.MountDevicesIntoContainer), slog.Any("ports", ports), slog.Any("required_devices", idxBrick.RequiredDevices))
		if idxBrick.MountDevicesIntoContainer {
			for i := range svcs {
				svcs[i].requireDevices = true
			}
		}

		composeFiles.AddIfMissing(composeFilePath)
		services = append(services, svcs...)
	}

	if len(app.Descriptor.RequiredDevices) > 0 { // nolint:staticcheck
		slog.Warn("The 'required_devices' field is deprecated. Please move requirements to the specific 'bricks' section.")
	}

	// Add the singleton services compose files to the list of the brick compose files
	for _, s := range brickServices {
		serviceCompose, ok := s.GetComposeFile()
		if !ok {
			slog.Error("service compose not found", slog.String("service_id", s.ServiceID))
			continue
		}
		svcs, err := extractServicesFromComposeFile(serviceCompose)
		if err != nil {
			slog.Error("loading service_compose", slog.String("service_id", s.ServiceID), slog.String("path", serviceCompose.String()), slog.Any("error", err))
			continue
		}
		composeFiles.AddIfMissing(serviceCompose)
		services = append(services, svcs...)
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
		{
			Type:   "bind",
			Source: "/dev",
			Target: "/dev",
		},
		{
			Type:     "bind",
			Source:   "/run/udev",
			Target:   "/run/udev",
			ReadOnly: true,
		},
	}

	for _, p := range cfg.RequiredRuntimesPaths() {
		volumes = append(volumes, volume{
			Type:   "bind",
			Source: p.String(),
			Target: p.String(),
		})
	}

	volumes = addLedControl(platform, volumes)
	groups := lookupGroups("video", "audio", "render", "dialout")
	// Support for NPU
	groups = append(groups, lookupGroups("fastrpc", "dmaheap")...)
	// Support GPIO access
	groups = append(groups, lookupGroups("gpiod")...)

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

	cgroupDrivers := []string{"drm", "dma_heap", "media", "video4linux", "alsa", "ttyUSB", "ttyACM"}
	deviceCgroupsRules := buildCgroupRules(cgroupDrivers)

	mainAppCompose.Services = &mainService{
		Main: service{
			Image:             pythonImage,
			Volumes:           volumes,
			Ports:             slices.Collect(maps.Keys(ports)),
			DeviceCgroupRules: deviceCgroupsRules,
			Entrypoint:        "/run.sh",
			DependsOn:         dependsOn,
			User:              getCurrentUser(),
			GroupAdd:          groups,
			ExtraHosts:        []string{"msgpack-rpc-router:host-gateway"},
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
	if err := generateServicesOverrideFile(app, services, getCurrentUser(), groups, overrideComposeFile, envs, deviceCgroupsRules); err != nil {
		return err
	}

	// Pre-provision containers required paths, if they do not exist.
	// This is required to preserve the host directory access rights for arduino user.
	// Otherwise, paths created by the container will have root:root ownership
	for _, additionalComposeFile := range composeFiles {
		composeFilePath := additionalComposeFile.String()
		slog.Debug("Pre-provisioning volumes from compose file", slog.String("compose_file", composeFilePath))
		provisionComposeVolumes(composeFilePath, app, envs)
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
	content, err := composeFile.ReadFile()
	if err != nil {
		return nil, err
	}

	prj, err := loader.LoadWithContext(
		context.Background(),
		types.ConfigDetails{
			ConfigFiles: []types.ConfigFile{{Filename: composeFile.String(), Content: content}},
			WorkingDir:  composeFile.Parent().String(),
			Environment: types.NewMapping(os.Environ()),
		},
		func(o *loader.Options) { o.SetProjectName("default", false); o.SkipConsistencyCheck = true },
		loader.WithSkipValidation,
	)
	if err != nil {
		return nil, err
	}

	services := make([]serviceInfo, 0, len(prj.Services))
	for name, svc := range prj.Services {
		hasHealthcheck := svc.HealthCheck != nil && len(svc.HealthCheck.Test) > 0
		var userPtr *string
		if svc.User != "" {
			userPtr = new(svc.User)
		}
		services = append(services, serviceInfo{
			name:           name,
			hasHealthcheck: hasHealthcheck,
			user:           userPtr,
		})
	}
	return services, nil
}

func generateServicesOverrideFile(arduinoApp *app.ArduinoApp, services []serviceInfo, user string, groups []uint32, overrideComposeFile *paths.Path, envs helpers.EnvVars, deviceCgroupsRules []string) error {
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
		User              *string           `yaml:"user,omitempty"`
		Volumes           *[]volume         `yaml:"volumes,omitempty"`
		DeviceCgroupRules *[]string         `yaml:"device_cgroup_rules,omitempty"`
		GroupAdd          *[]uint32         `yaml:"group_add,omitempty"`
		Labels            map[string]string `yaml:"labels,omitempty"`
		Environment       map[string]string `yaml:"environment,omitempty"`
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
			override.DeviceCgroupRules = &deviceCgroupsRules
			devVolumes := []volume{
				{Type: "bind", Source: "/dev", Target: "/dev"},
			}
			override.Volumes = &devVolumes
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

// provisionComposeVolumes ensures we create the parent folder with the correct owner.
// By default docker if it doesn't find the folder, it will create it as root.
// We do not want that, to make sure to have it as `arduino:arduino` we have
// to manually parse the volumes, and make sure to create the target dirs ourself.
func provisionComposeVolumes(composeFilePath string, arduinoApp *app.ArduinoApp, mappedEnv map[string]string) {
	content, err := os.ReadFile(composeFilePath)
	if err != nil {
		slog.Warn("Failed to read compose file", slog.String("compose_file", composeFilePath), slog.Any("error", err))
		return
	}

	env := types.NewMapping(os.Environ())
	env["APP_HOME"] = arduinoApp.FullPath.String()
	for k, v := range mappedEnv {
		env[k] = v
	}

	prj, err := loader.LoadWithContext(
		context.Background(),
		types.ConfigDetails{
			ConfigFiles: []types.ConfigFile{{Filename: composeFilePath, Content: content}},
			WorkingDir:  paths.New(composeFilePath).Parent().String(),
			Environment: env,
		},
		func(o *loader.Options) { o.SetProjectName("default", false); o.SkipConsistencyCheck = true },
		loader.WithSkipValidation,
	)
	if err != nil {
		slog.Warn("Failed to parse compose file for volume provisioning", slog.String("compose_file", composeFilePath), slog.Any("error", err))
		return
	}

	for _, svc := range prj.Services {
		for _, v := range svc.Volumes {
			if v.Type != types.VolumeTypeBind {
				continue
			}
			hostDirectory := paths.New(v.Source)
			if !hostDirectory.Exist() {
				if err := hostDirectory.MkdirAll(); err != nil {
					slog.Warn("Failed to create host directory for compose file", slog.String("compose_file", composeFilePath), slog.String("host_directory", hostDirectory.String()), slog.Any("error", err))
				} else {
					slog.Debug("Pre-provisioning host directory for compose file", slog.String("compose_file", composeFilePath), slog.String("host_directory", hostDirectory.String()))
				}
			}
		}
	}
}

func buildCgroupRules(drivers []string) []string {
	var rules []string

	for _, driver := range drivers {
		major, err := resolveMajorNumber(driver)
		if err != nil {
			slog.Warn("could not resolve major number, skipping cgroup rule",
				slog.String("driver", driver),
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
