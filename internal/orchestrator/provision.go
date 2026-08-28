// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/containerd/errdefs"
	"github.com/docker/cli/cli/command"
	"github.com/docker/docker/api/types/container"

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

// BuildOptions is what the generation cannot derive from the app: the caller states it.
type BuildOptions struct {
	// ProjectName is the docker compose project the app runs as.
	ProjectName string
	// ComposesDir is where the brick and service composes are copied to, to be included
	// by a relative path. Nil includes them from the asset dir of this host.
	ComposesDir *paths.Path
	// Labels are added to the main service.
	Labels map[string]string
}

// Resolve turns the app bricks and services into the compose templates it is started
// from, deriving them from the app and the target board and never from this host.
func (p *Provision) Resolve(
	arduinoApp *app.ArduinoApp,
	genPath *paths.Path,
	bricksIndex *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
	cfg config.Configuration,
	appEnv types.Mapping,
	platform platform.Platform,
	opts BuildOptions,
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

	return generateComposeTemplate(arduinoApp, genPath, bricksIndex, servicesIndex, p.pythonImage, cfg, appEnv, platform, opts)
}

// Render evaluates the templates against the board the app is being started on and
// writes the single compose file docker is given.
func (p *Provision) Render(
	ctx context.Context,
	arduinoApp *app.ArduinoApp,
	env types.Mapping,
	secrets types.Mapping,
) error {
	if arduinoApp == nil {
		return fmt.Errorf("provisioning failed: arduinoApp is nil")
	}

	if arduinoApp.AppComposeTemplateFilePath().NotExist() {
		return fmt.Errorf("provisioning failed: %s not found, the app was not resolved", app.MainTemplateFileName)
	}

	prj, err := renderComposeFile(ctx, arduinoApp, env, secrets)
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
