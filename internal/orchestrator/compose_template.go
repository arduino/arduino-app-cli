// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"cmp"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/types"
	yaml "github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

// A compose template is what Provision.Resolve writes: it states what the app needs of
// a board by name, as {{ }} expressions, and of its environment as ${VAR}.

type bindOptions struct {
	CreateHostPath bool `yaml:"create_host_path" json:"create_host_path"`
}

func generateComposeTemplate(
	arduinoApp *app.ArduinoApp,
	genPath *paths.Path,
	bricksIndex *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
	pythonImage string,
	cfg config.Configuration,
	appEnv types.Mapping,
	platform platform.Platform,
	opts BuildOptions,
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
	composeProjectName := opts.ProjectName
	if composeProjectName == "" {
		name, err := getAppComposeProjectNameFromApp(*arduinoApp, cfg)
		if err != nil {
			return err
		}
		composeProjectName = name
	}
	mainAppCompose.Name = composeProjectName

	includes, err := composeIncludes(composeFiles, genPath, cfg, opts)
	if err != nil {
		return err
	}
	mainAppCompose.Include = includes

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
		Labels:            mainServiceLabels(opts),
		Environment:       templateEnvironment(appEnv),
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
	for name, override := range servicesOverrides(services, appUserExpr, appEnv, deviceDrivers, groupNames) {
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

func mainServiceLabels(opts BuildOptions) map[string]string {
	labels := map[string]string{
		DockerAppLabel:     "true",
		DockerAppMainLabel: "true",
		DockerAppPathLabel: appHomeRef,
	}
	maps.Insert(labels, maps.All(opts.Labels))
	return labels
}

// composeIncludes is the include: list of the generated template. It points at the
// asset dir, unless a dir is given to copy the brick and service composes to, which
// is what makes a release carry its own compose set.
func composeIncludes(composeFiles paths.PathList, genPath *paths.Path, cfg config.Configuration, opts BuildOptions) ([]string, error) {
	if opts.ComposesDir == nil {
		return composeFiles.AsStrings(), nil
	}

	includes := make([]string, 0, len(composeFiles))
	for _, composeFile := range composeFiles {
		// Keep the layout of the asset dir, which is what makes the copied names
		// unique: every brick has its own brick_compose.yaml.
		relPath, err := composeFile.RelFrom(cfg.AssetDir())
		if err != nil {
			// A brick shipped by the app itself: keep the folder holding it.
			relPath = paths.New(composeFile.Parent().Base(), composeFile.Base())
		}

		dst := opts.ComposesDir.JoinPath(relPath)
		if err := dst.Parent().MkdirAll(); err != nil {
			return nil, fmt.Errorf("failed to create the compose dir %s: %w", dst.Parent(), err)
		}
		if err := composeFile.CopyTo(dst); err != nil {
			return nil, fmt.Errorf("failed to copy the compose file %s: %w", composeFile, err)
		}

		// docker compose resolves a relative include from the project directory,
		// which is the one holding the generated file.
		include, err := dst.RelFrom(genPath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve the include path of %s: %w", dst, err)
		}
		includes = append(includes, include.String())
	}
	return includes, nil
}

// provisionComposeVolumes creates the bind sources of the services, which docker would
// otherwise create as root.
// servicesOverrides is what to apply to the services the brick and service composes
// declare: they are not ours, so only these fields are stated.
func servicesOverrides(services []serviceInfo, user string, appEnv types.Mapping, deviceDrivers, groupNames []string) map[string]any {
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
			Environment: templateEnvironment(appEnv),
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

// templateEnvironment is the `environment:` section of a template: the values the app
// is wired with, plus a reference to every host fact, resolved when it is rendered.
func templateEnvironment(appEnv types.Mapping) types.Mapping {
	env := appEnv.Clone()
	for name := range hostVariables {
		env[name] = "${" + name + "}"
	}
	// A brick can declare a VIDEO_DEVICE of its own, and it has to keep applying
	// when there is no camera to detect on the board.
	if brickValue, ok := appEnv["VIDEO_DEVICE"]; ok {
		env["VIDEO_DEVICE"] = fmt.Sprintf("${VIDEO_DEVICE:-%s}", brickValue)
	}
	return env
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
