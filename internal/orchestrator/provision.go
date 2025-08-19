package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/containerd/errdefs"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/store"

	"github.com/arduino/go-paths-helper"
	"github.com/docker/cli/cli/command"
	"github.com/docker/docker/api/types/container"
	dockerClient "github.com/docker/docker/client"
	yaml "github.com/goccy/go-yaml"
)

type volume struct {
	Type   string `yaml:"type"`
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

type service struct {
	Image      string            `yaml:"image"`
	DependsOn  []string          `yaml:"depends_on,omitempty"`
	Volumes    []volume          `yaml:"volumes"`
	Devices    []string          `yaml:"devices"`
	Ports      []string          `yaml:"ports"`
	User       string            `yaml:"user"`
	GroupAdd   []string          `yaml:"group_add"`
	Entrypoint string            `yaml:"entrypoint"`
	ExtraHosts []string          `yaml:"extra_hosts,omitempty"`
	Labels     map[string]string `yaml:"labels,omitempty"`
}

func ProvisionApp(
	ctx context.Context,
	provisioner *Provision,
	bricksIndex *bricksindex.BricksIndex,
	mapped_env map[string]string,
	app *app.ArduinoApp,
) error {
	start := time.Now()
	defer func() {
		slog.Info("Provisioning took", "duration", time.Since(start).String())
	}()
	return provisioner.App(ctx, bricksIndex, app, mapped_env)
}

type Provision struct {
	docker              command.Cli
	useDynamicProvision bool
	staticStore         *store.StaticStore
	pythonImage         string
}

func NewProvision(
	docker command.Cli,
	staticStore *store.StaticStore,
	useDynamicProvision bool,
	pythonImage string,
) (*Provision, error) {
	if useDynamicProvision {
		if err := dynamicProvisioning(context.Background(), docker.Client(), pythonImage, paths.TempDir().String()); err != nil {
			return nil, fmt.Errorf("failed to perform dynamic provisioning: %w", err)
		}
	}

	return &Provision{
		docker:              docker,
		staticStore:         staticStore,
		useDynamicProvision: useDynamicProvision,
		pythonImage:         pythonImage,
	}, nil
}

func (p *Provision) App(
	ctx context.Context,
	bricksIndex *bricksindex.BricksIndex,
	arduinoApp *app.ArduinoApp,
	mapped_env map[string]string,
) error {
	if arduinoApp == nil {
		return fmt.Errorf("provisioning failed: arduinoApp is nil")
	}

	dst := arduinoApp.ProvisioningStateDir().Join("compose")
	if err := dst.RemoveAll(); err != nil {
		return fmt.Errorf("failed to remove compose directory: %w", err)
	}
	if p.useDynamicProvision {
		composeDir := paths.New(paths.TempDir().String(), ".cache", "compose")
		if err := composeDir.CopyDirTo(dst); err != nil {
			return fmt.Errorf("failed to copy compose directory: %w", err)
		}
	} else {
		if err := p.staticStore.SaveComposeFolderTo(dst.String()); err != nil {
			return fmt.Errorf("failed to save compose folder: %w", err)
		}
	}

	return generateMainComposeFile(arduinoApp, bricksIndex, p.pythonImage, mapped_env)
}

func (p *Provision) IsUsingDynamicProvision() bool {
	return p.useDynamicProvision
}

func (p *Provision) DynamicProvisionDir() *paths.Path {
	if p.useDynamicProvision {
		return paths.TempDir().Join(".cache")
	}
	return nil
}

func dynamicProvisioning(
	ctx context.Context,
	docker dockerClient.APIClient,
	pythonImage, srcPath string,
) error {
	containerCfg := &container.Config{
		Image: pythonImage,
		User:  getCurrentUser(),
		Entrypoint: []string{
			"/bin/bash",
			"-c",
			fmt.Sprintf("%s && %s",
				"arduino-bricks-list-modules --provision-compose",
				"arduino-bricks-list-modules -o /app/.cache/bricks-list.yaml -m /app/.cache/models-list.yaml",
			),
		},
	}
	containerHostCfg := &container.HostConfig{
		Binds:      []string{srcPath + ":/app"},
		AutoRemove: true,
	}
	resp, err := docker.ContainerCreate(ctx, containerCfg, containerHostCfg, nil, nil, "")
	if err != nil {
		if errors.Is(err, errdefs.ErrNotFound) {
			if err := pullBasePythonContainer(ctx, pythonImage); err != nil {
				return fmt.Errorf("provisioning failed to pull base image: %w", err)
			}
			// Now that we have pulled the container we recreate it
			resp, err = docker.ContainerCreate(ctx, containerCfg, containerHostCfg, nil, nil, "")
		}
		if err != nil {
			return fmt.Errorf("provisiong failed to create container: %w", err)
		}
	}

	slog.Debug("provisioning container created", slog.String("container_id", resp.ID))

	waitCh, errCh := docker.ContainerWait(ctx, resp.ID, container.WaitConditionNextExit)
	if err := docker.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
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

const DockerAppLabel = "cc.arduino.app"
const DockerAppPathLabel = "cc.arduino.app.path"

func generateMainComposeFile(
	app *app.ArduinoApp,
	bricksIndex *bricksindex.BricksIndex,
	pythonImage string,
	mapped_env map[string]string,
) error {
	slog.Debug("Generating main compose file for the App")

	var composeFiles paths.PathList
	services := []string{}
	brickServices := map[string][]string{}
	for _, brick := range app.Descriptor.Bricks {
		brickPath := filepath.Join(strings.Split(brick.ID, ":")...)
		composeFilePath := app.ProvisioningStateDir().Join("compose", brickPath, "brick_compose.yaml")
		if composeFilePath.Exist() {
			composeFiles.Add(composeFilePath)
			svcs, e := extracServicesFromComposeFile(composeFilePath)
			if e != nil {
				slog.Error("Failed to extract services from compose file", slog.String("compose_file", composeFilePath.String()), slog.Any("error", e))
				continue
			}
			brickServices[brick.ID] = svcs
			services = append(services, svcs...)
			slog.Debug("Brick compose file found", slog.String("module", brick.ID), slog.String("path", composeFilePath.String()))
		} else {
			slog.Debug("Brick compose file not found", slog.String("module", brick.ID), slog.String("path", composeFilePath.String()))
		}
	}

	// Create a single docker-mainCompose that includes all the required services
	mainComposeFile := app.AppComposeFilePath()
	// If required, create an override compose file for devices
	overrideComposeFile := app.ProvisioningStateDir().Join("app-compose-overrides.yaml")

	type mainService struct {
		Main service `yaml:"main"`
	}
	var mainAppCompose struct {
		Name     string       `yaml:"name"`
		Include  []string     `yaml:"include,omitempty"`
		Services *mainService `yaml:"services,omitempty"`
	}
	writeMainCompose := func() error {
		data, err := yaml.Marshal(mainAppCompose)
		if err != nil {
			return err
		}
		if err := mainComposeFile.WriteFile(data); err != nil {
			return err
		}
		return nil
	}

	// Merge compose
	composeProjectName, err := getAppComposeProjectNameFromApp(*app)
	if err != nil {
		return err
	}
	mainAppCompose.Name = composeProjectName
	mainAppCompose.Include = composeFiles.AsStrings()
	if err := writeMainCompose(); err != nil {
		return err
	}

	slog.Debug("Compose file for the App created", slog.String("compose_file", mainComposeFile.String()))

	ports := make(map[string]struct{}, len(app.Descriptor.Ports))
	for _, p := range app.Descriptor.Ports {
		ports[fmt.Sprintf("%d:%d", p, p)] = struct{}{}
	}

	servicesThatRequireDevices := []string{}
	for _, b := range app.Descriptor.Bricks {
		brick, found := bricksIndex.FindBrickByID(b.ID)
		slog.Debug("Processing brick", slog.String("brick_id", b.ID), slog.Bool("found", found))
		if !found {
			continue
		}
		for _, p := range brick.Ports {
			ports[fmt.Sprintf("%s:%s", p, p)] = struct{}{}
		}
		slog.Debug("Brick require Devices", slog.Bool("Devices", brick.RequiresDevices), slog.Any("ports", ports))
		if brick.RequiresDevices {
			// Load services from compose file
			if svcs, ok := brickServices[b.ID]; ok {
				servicesThatRequireDevices = append(servicesThatRequireDevices, svcs...)
			} else {
				slog.Debug("(RequiresDevices) No compose file found for brick", slog.String("brick_id", b.ID))
			}
		}
	}

	volumes := []volume{
		{
			Type:   "bind",
			Source: app.FullPath.String(),
			Target: "/app",
		},
	}

	slog.Debug("Adding UNIX socket", slog.Any("sock", orchestratorConfig.RouterSocketPath().String()), slog.Bool("exists", orchestratorConfig.RouterSocketPath().Exist()))
	if orchestratorConfig.RouterSocketPath().Exist() {
		volumes = append(volumes, volume{
			Type:   "bind",
			Source: orchestratorConfig.RouterSocketPath().String(),
			Target: "/var/run/arduino-router.sock",
		})
	}

	devices := getDevices()
	groups := []string{"dialout", "video", "audio"}

	mainAppCompose.Services = &mainService{
		Main: service{
			Image:      pythonImage,
			Volumes:    volumes,
			Ports:      slices.Collect(maps.Keys(ports)),
			Devices:    devices,
			Entrypoint: "/run.sh",
			DependsOn:  services,
			User:       getCurrentUser(),
			GroupAdd:   groups,
			ExtraHosts: []string{"msgpack-rpc-router:host-gateway"},
			Labels: map[string]string{
				DockerAppLabel:     "true",
				DockerAppPathLabel: app.FullPath.String(),
			},
		},
	}

	// Write the main compose file
	if e := writeMainCompose(); e != nil {
		return e
	}

	// If there are services that require devices, we need to generate an override compose file
	// Write additional file to override devices section in included compose files
	if e := generateServicesOverrideFile(services, servicesThatRequireDevices, devices, getCurrentUser(), groups, overrideComposeFile); e != nil {
		return e
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
		provisionComposeVolumes(composeFilePath, volumes, app, mapped_env)
	}

	// Done!
	return nil
}

func extracServicesFromComposeFile(composeFile *paths.Path) ([]string, error) {
	if content, e := os.ReadFile(composeFile.String()); e != nil {
		return nil, e
	} else {
		servicesThatRequireDevices := []string{}
		type serviceMin struct {
			Image string `yaml:"image"`
		}
		type composeServices struct {
			Services map[string]serviceMin `yaml:"services"`
		}
		var index composeServices
		if err := yaml.Unmarshal(content, &index); err != nil {
			return nil, err
		}
		if len(index.Services) > 0 {
			for svc := range index.Services {
				servicesThatRequireDevices = append(servicesThatRequireDevices, svc)
			}
		}
		return servicesThatRequireDevices, nil
	}
}

func generateServicesOverrideFile(services []string, servicesThatRequireDevices []string, devices []string, user string, groups []string, overrideComposeFile *paths.Path) error {
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
		Devices  *[]string `yaml:"devices,omitempty"`
		User     string    `yaml:"user"`
		GroupAdd []string  `yaml:"group_add"`
	}
	var overrideCompose struct {
		Services map[string]serviceOverride `yaml:"services,omitempty"`
	}
	overrideCompose.Services = make(map[string]serviceOverride, len(services))
	for _, svc := range services {
		override := serviceOverride{
			User:     user,
			GroupAdd: groups,
		}
		if slices.Contains(servicesThatRequireDevices, svc) {
			override.Devices = &devices
		}
		overrideCompose.Services[svc] = override
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
	volumePathReplaceRE    = regexp.MustCompile(`\$\{([A-Z_-]+)(:-)?([\/a-zA-Z0-9._-]+)?\}`)
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
				volume = volumePathReplaceRE.ReplaceAllString(volume, groups[3])
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
