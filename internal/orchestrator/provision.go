// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
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

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

type volume struct {
	Type     string       `yaml:"type" json:"type"`
	Source   string       `yaml:"source" json:"source"`
	Target   string       `yaml:"target" json:"target"`
	ReadOnly bool         `yaml:"read_only,omitempty" json:"read_only,omitempty"`
	Bind     *bindOptions `yaml:"bind,omitempty" json:"bind,omitempty"`
}

type bindOptions struct {
	CreateHostPath bool `yaml:"create_host_path" json:"create_host_path"`
}

type dependsOnCondition struct {
	Condition string `yaml:"condition"`
}

type logging struct {
	Driver  string            `yaml:"driver"`
	Options map[string]string `yaml:"options,omitempty"`
}

// A field holding an expression is a string here: what it renders to is a list.
type service struct {
	Image             string                        `yaml:"image"`
	DependsOn         map[string]dependsOnCondition `yaml:"depends_on,omitempty"`
	Volumes           []any                         `yaml:"volumes"`
	Ports             []string                      `yaml:"ports"`
	User              string                        `yaml:"user"`
	GroupAdd          []string                      `yaml:"group_add,omitempty"`
	DeviceCgroupRules []string                      `yaml:"device_cgroup_rules,omitempty"`
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

// Resolve turns the app bricks and services into the compose templates it is started
// from, deriving them from the app and the target board and never from this host.
func (p *Provision) Resolve(
	genPath *paths.Path,
	bricksIndex *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
	arduinoApp *app.ArduinoApp,
	cfg config.Configuration,
	env AppEnv,
	platform platform.Platform,
) error {
	if arduinoApp == nil {
		return fmt.Errorf("provisioning failed: arduinoApp is nil")
	}

	// genPath is the .cache of the app, or the prebuild dir of a release.
	if genPath.NotExist() {
		if err := genPath.MkdirAll(); err != nil {
			return fmt.Errorf("provisioning failed: unable to create %s: %w", genPath, err)
		}
	}

	bricksIndex = bricksIndex.WithAppBricks(arduinoApp.LocalBricks)

	return generateMainComposeTemplate(arduinoApp, genPath, bricksIndex, servicesIndex, p.pythonImage, cfg, env, platform)
}

// Render evaluates the templates against the board the app is being started on and
// writes the single compose file docker is given.
func (p *Provision) Render(
	ctx context.Context,
	arduinoApp *app.ArduinoApp,
	env AppEnv,
) error {
	if arduinoApp == nil {
		return fmt.Errorf("provisioning failed: arduinoApp is nil")
	}

	if arduinoApp.AppComposeTemplateFilePath().NotExist() {
		return fmt.Errorf("provisioning failed: %s not found, the app was not resolved", app.MainTemplateFileName)
	}

	prj, err := renderComposeFile(ctx, arduinoApp, env)
	if err != nil {
		return fmt.Errorf("provisioning failed to render the app compose file: %w", err)
	}

	provisionComposeVolumes(prj)
	return nil
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

func generateMainComposeTemplate(
	arduinoApp *app.ArduinoApp,
	genPath *paths.Path,
	bricksIndex *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
	pythonImage string,
	cfg config.Configuration,
	envs AppEnv,
	platform platform.Platform,
) error {
	slog.Debug("Generating main compose file for the App")

	ports := make(map[string]struct{}, len(arduinoApp.Descriptor.Ports))
	for _, p := range arduinoApp.Descriptor.Ports {
		ports[fmt.Sprintf("%d:%d", p, p)] = struct{}{}
	}

	brickServices := make(map[string]servicesindex.Service)
	var composeFiles paths.PathList
	services := make([]serviceInfo, 0, len(arduinoApp.Descriptor.Bricks))
	for _, brick := range arduinoApp.Descriptor.Bricks {
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

	if len(arduinoApp.Descriptor.RequiredDevices) > 0 { // nolint:staticcheck
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

	var mainAppCompose struct {
		Name     string         `yaml:"name"`
		Include  []string       `yaml:"include,omitempty"`
		Services map[string]any `yaml:"services,omitempty"`
	}
	// Merge compose
	composeProjectName, err := getAppComposeProjectNameFromApp(*arduinoApp, cfg)
	if err != nil {
		return err
	}
	mainAppCompose.Name = composeProjectName
	mainAppCompose.Include = composeFiles.AsStrings()

	volumes := []any{
		volume{
			Type:   "bind",
			Source: appHomeRef,
			Target: "/app",
		},
		volume{
			Type:   "bind",
			Source: "/dev",
			Target: "/dev",
		},
	}

	// Mounted only where the board has them.
	optionalMounts := slices.Concat(
		[]string{"/run/udev:ro", "/run/user/1000/pipewire-0"},
		// camx CSI cameras are accessed through the cam_server socket and a host userspace library
		[]string{"/run/cam_server", "/usr/lib/libcamera_metadata.so.0.1.0"},
		platform.Linux.BoardLeds.AsStrings(),
	)
	for _, mount := range optionalMounts {
		expr, err := mountExpr(mount)
		if err != nil {
			return err
		}
		volumes = append(volumes, expr)
	}

	// The required runtime sockets, at whichever of their paths the board has, and
	// the groups needed to access them.
	var runtimeGroupNames []string
	for _, runtime := range cfg.RequiredRuntimeCandidates() {
		for _, path := range runtime.Paths {
			expr, err := mountExpr(path)
			if err != nil {
				return err
			}
			volumes = append(volumes, expr)
		}
		if runtime.Group != "" && !slices.Contains(runtimeGroupNames, runtime.Group) {
			runtimeGroupNames = append(runtimeGroupNames, runtime.Group)
		}
	}

	groupNames := slices.Concat(
		[]string{"video", "audio", "render", "dialout"},
		runtimeGroupNames,              // access to the required runtime sockets
		[]string{"fastrpc", "dmaheap"}, // support for NPU
		[]string{"gpiod"},              // support GPIO access
	)

	// A service with a healthcheck is waited for, the others only have to be started.
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

	deviceDrivers := []string{"drm", "dma_heap", "media", "video4linux", "alsa", "ttyUSB", "ttyACM"}

	mainAppCompose.Services = map[string]any{"main": service{
		Image:             pythonImage,
		Volumes:           volumes,
		Ports:             slices.Collect(maps.Keys(ports)),
		Entrypoint:        "/run.sh",
		DependsOn:         dependsOn,
		User:              appUserExpr,
		GroupAdd:          groupExprs(groupNames),
		DeviceCgroupRules: cgroupRuleExprs(deviceDrivers),
		ExtraHosts:        []string{"msgpack-rpc-router:host-gateway"},
		Labels: map[string]string{
			DockerAppLabel:     "true",
			DockerAppMainLabel: "true",
			DockerAppPathLabel: appHomeRef,
		},
		Environment: envs.BuildTime(),
		Logging: &logging{
			Driver: "json-file",
			Options: map[string]string{
				"max-size": "5m",
				"max-file": "2",
			},
		},
	},
	}

	// The services the included composes declare are overridden here: what a file
	// includes is merged under its own services.
	for name, override := range servicesOverrides(services, appUserExpr, envs, deviceDrivers, groupNames) {
		mainAppCompose.Services[name] = override
	}

	// Write the main compose file
	data, err := yaml.Marshal(mainAppCompose)
	if err != nil {
		return err
	}
	mainTemplateFile := genPath.Join(app.MainTemplateFileName)
	if err := mainTemplateFile.WriteFile(data); err != nil {
		return err
	}

	// Done!
	return nil
}

type serviceInfo struct {
	name           string
	hasHealthcheck bool
	user           *string
	requireDevices bool
}

// extractServicesFromComposeFile reads what a brick or service compose declares: its
// services, their healthcheck and their user. No variable takes part, none is resolved.
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
			Environment: types.Mapping{},
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

// servicesOverrides is what to apply to the services the brick and service composes
// declare: they are not ours, so only these fields are stated.
func servicesOverrides(services []serviceInfo, user string, envs AppEnv, deviceDrivers, groupNames []string) map[string]any {
	type serviceOverride struct {
		User              *string           `yaml:"user,omitempty"`
		Volumes           []volume          `yaml:"volumes,omitempty"`
		GroupAdd          []string          `yaml:"group_add,omitempty"`
		DeviceCgroupRules []string          `yaml:"device_cgroup_rules,omitempty"`
		Labels            map[string]string `yaml:"labels,omitempty"`
		Environment       types.Mapping     `yaml:"environment,omitempty"`
	}

	overrides := make(map[string]any, len(services))
	for _, svc := range services {
		override := serviceOverride{
			GroupAdd: groupExprs(groupNames),
			Labels: map[string]string{
				DockerAppLabel:     "true",
				DockerAppPathLabel: appHomeRef,
			},
			Environment: envs.BuildTime(),
		}
		// If service defines a user, do not override it
		if svc.user == nil {
			override.User = &user
		}
		if svc.requireDevices {
			override.DeviceCgroupRules = cgroupRuleExprs(deviceDrivers)
			override.Volumes = []volume{{Type: "bind", Source: "/dev", Target: "/dev"}}
		}
		overrides[svc.name] = override
	}
	return overrides
}

// provisionComposeVolumes creates the bind sources of the services, which docker would
// otherwise create as root.
func provisionComposeVolumes(prj *types.Project) {
	for name, svc := range prj.Services {
		if name == "main" {
			continue
		}
		for _, v := range svc.Volumes {
			if v.Type != types.VolumeTypeBind {
				continue
			}
			hostDirectory := paths.New(v.Source)
			if !hostDirectory.Exist() {
				if err := hostDirectory.MkdirAll(); err != nil {
					slog.Warn("Failed to create host directory for compose file", slog.String("service", name), slog.String("host_directory", hostDirectory.String()), slog.Any("error", err))
				} else {
					slog.Debug("Pre-provisioning host directory for compose file", slog.String("service", name), slog.String("host_directory", hostDirectory.String()))
				}
			}
		}
	}
}

// mountExpr binds a path where it is, `<path>:ro` read-only. It renders to nothing,
// and so is dropped, on a board that has not the path: never created, being optional.
func mountExpr(mount string) (string, error) {
	source, option, _ := strings.Cut(mount, ":")
	bind, err := json.Marshal(volume{
		Type:     "bind",
		Source:   source,
		Target:   source,
		ReadOnly: option == "ro",
		Bind:     &bindOptions{CreateHostPath: false},
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("{{ if pathExists %s }}%s{{ end }}", strconv.Quote(source), bind), nil
}

func groupExprs(names []string) []string {
	exprs := make([]string, 0, len(names))
	for _, name := range names {
		exprs = append(exprs, fmt.Sprintf("{{ groupID %s }}", strconv.Quote(name)))
	}
	return exprs
}

func cgroupRuleExprs(drivers []string) []string {
	exprs := make([]string, 0, len(drivers))
	for _, driver := range drivers {
		exprs = append(exprs, fmt.Sprintf("{{ with deviceMajor %s }}c {{ . }}:* rmw{{ end }}", strconv.Quote(driver)))
	}
	return exprs
}
