// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"

	"github.com/arduino/arduino-app-cli/internal/dockerhelper"
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
	appsDir     *paths.Path
}

func NewProvision(
	docker command.Cli,
	cfg config.Configuration,
) (*Provision, error) {
	provision := &Provision{
		docker:      docker,
		pythonImage: cfg.PythonImage,
		appsDir:     cfg.AppsDir(),
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
	arduinoApp *app.ArduinoApp,
	genPath *paths.Path,
	bricksIndex *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
	cfg config.Configuration,
	appEnv types.Mapping,
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

	return generateComposeTemplate(arduinoApp, genPath, bricksIndex, servicesIndex, p.pythonImage, cfg, appEnv, platform)
}

// Render evaluates the templates against the board the app is being started on and
// writes the single compose file docker is given.
func (p *Provision) Render(
	ctx context.Context,
	arduinoApp *app.ArduinoApp,
	env types.Mapping,
	secrets types.Mapping,
) (*types.Project, error) {
	if arduinoApp == nil {
		return nil, fmt.Errorf("provisioning failed: arduinoApp is nil")
	}

	if arduinoApp.AppComposeTemplateFilePath().NotExist() {
		return nil, fmt.Errorf("provisioning failed: %s not found, the app was not resolved", app.MainTemplateFileName)
	}

	// The docker project names what docker creates, so it is the app installed here and
	// not the version of it: an upgrade must replace the containers of the one before.
	projectName := composeProjectName(arduinoApp.FullPath, p.appsDir)

	prj, err := renderComposeFile(ctx, arduinoApp, env, secrets, projectName)
	if err != nil {
		return nil, fmt.Errorf("provisioning failed to render the app compose file: %w", err)
	}

	provisionComposeVolumes(prj)
	return prj, nil
}

func (p *Provision) init(srcPath string) error {
	return dockerhelper.Run(context.Background(), p.docker.Client(), dockerhelper.RunOptions{
		Image: p.pythonImage,
		Entrypoint: []string{"/bin/bash", "-c", fmt.Sprintf("%s && %s",
			"arduino-bricks-list-modules -o /app/bricks-list.yaml -m /app/models-list.yaml",
			"arduino-bricks-list-modules --provision-compose -o /app",
		)},
		Binds: []string{srcPath + ":/app"},
	})
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
