package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/arduino/go-paths-helper"
	"github.com/containerd/errdefs"
	"github.com/docker/cli/cli/command"
	"github.com/docker/docker/api/types/container"
	yaml "github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/store"
	"github.com/arduino/arduino-app-cli/pkg/helpers"
)

type volume struct {
	Type   string `yaml:"type"`
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

type dependsOnCondition struct {
	Condition string `yaml:"condition"`
}

type service struct {
	Image       string                        `yaml:"image"`
	DependsOn   map[string]dependsOnCondition `yaml:"depends_on,omitempty"`
	Volumes     []volume                      `yaml:"volumes"`
	Devices     []string                      `yaml:"devices"`
	Ports       []string                      `yaml:"ports"`
	User        string                        `yaml:"user"`
	GroupAdd    []string                      `yaml:"group_add"`
	Entrypoint  string                        `yaml:"entrypoint"`
	ExtraHosts  []string                      `yaml:"extra_hosts,omitempty"`
	Labels      map[string]string             `yaml:"labels,omitempty"`
	Environment map[string]string             `yaml:"environment,omitempty"`
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
) error {
	if arduinoApp == nil {
		return fmt.Errorf("provisioning failed: arduinoApp is nil")
	}

	if arduinoApp.ProvisioningStateDir().NotExist() {
		if err := arduinoApp.ProvisioningStateDir().MkdirAll(); err != nil {
			return fmt.Errorf("provisioning failed: unable to create .cache")
		}
	}

	return generateMainComposeFile(arduinoApp, bricksIndex, p.pythonImage, cfg, mapped_env, staticStore)
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
	mapped_env map[string]string,
	staticStore *store.StaticStore,
) error {
	slog.Debug("Generating main compose file for the App")

	ports := make(map[string]struct{}, len(app.Descriptor.Ports))
	for _, p := range app.Descriptor.Ports {
		ports[fmt.Sprintf("%d:%d", p, p)] = struct{}{}
	}

	var composeFiles paths.PathList
	services := make(map[string]serviceInfo)
	var servicesThatRequireDevices []string
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

		// 2. Retrieve the brick_compose.yaml file.
		composeFilePath, err := staticStore.GetBrickComposeFilePathFromID(brick.ID)
		if err != nil {
			slog.Error("brick compose id not valid", slog.String("error", err.Error()), slog.String("brick_id", brick.ID))
			continue
		}

		// 3. Retrieve the compose services names.
		svcs, err := extractServicesFromComposeFile(composeFilePath)
		if err != nil {
			slog.Error("loading brick_compose", slog.String("brick_id", brick.ID), slog.String("path", composeFilePath.String()), slog.Any("error", err))
			continue
		}

		// 4. Retrieve the required devices that we have to mount
		slog.Debug("Brick require Devices", slog.Bool("Devices", idxBrick.RequiresDevices), slog.Any("ports", ports))
		if idxBrick.RequiresDevices {
			servicesThatRequireDevices = slices.AppendSeq(servicesThatRequireDevices, maps.Keys(svcs))
		}

		composeFiles.Add(composeFilePath)
		maps.Insert(services, maps.All(svcs))
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

	devices := getDevices()
	if devices.hasVideoDevice {
		// If we are adding video devices, mount also /dev/v4l if it exists to allow access to by-id/path links
		if paths.New("/dev/v4l").Exist() {
			volumes = append(volumes, volume{
				Type:   "bind",
				Source: "/dev/v4l",
				Target: "/dev/v4l",
			})
		}
	}

	volumes = addLedControl(volumes)

	groups := []string{"dialout", "video", "audio", "render"}

	// Define depends_on conditions
	// Services with healthcheck will be started only when healthy
	// Services without healthcheck will be started as soon as the container is started
	dependsOn := make(map[string]dependsOnCondition, len(services))
	for name := range services {
		if services[name].hasHealthcheck {
			dependsOn[name] = dependsOnCondition{
				Condition: "service_healthy",
			}
		} else {
			dependsOn[name] = dependsOnCondition{
				Condition: "service_started",
			}
		}
	}

	mainAppCompose.Services = &mainService{
		Main: service{
			Image:      pythonImage,
			Volumes:    volumes,
			Ports:      slices.Collect(maps.Keys(ports)),
			Devices:    devices.devicePaths,
			Entrypoint: "/run.sh",
			DependsOn:  dependsOn,
			User:       getCurrentUser(),
			GroupAdd:   append(groups, "gpiod"),
			ExtraHosts: []string{"msgpack-rpc-router:host-gateway"},
			Labels: map[string]string{
				DockerAppLabel:     "true",
				DockerAppMainLabel: "true",
				DockerAppPathLabel: app.FullPath.String(),
			},
			// Add the host ip to the environment variables if we can retrieve it.
			// This is used to show the network ip address of the board in the apps.
			Environment: map[string]string{
				"HOST_IP": func() string {
					if hostIP, err := helpers.GetHostIP(); err != nil {
						slog.Warn("Failed to get host IP", slog.Any("error", err))
						return ""
					} else {
						return hostIP
					}
				}(),
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
	if e := generateServicesOverrideFile(app, slices.Collect(maps.Keys(services)), servicesThatRequireDevices, devices.devicePaths, getCurrentUser(), groups, overrideComposeFile); e != nil {
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

type serviceInfo struct {
	hasHealthcheck bool
}

func extractServicesFromComposeFile(composeFile *paths.Path) (map[string]serviceInfo, error) {
	content, err := os.ReadFile(composeFile.String())
	if err != nil {
		return nil, err
	}

	type serviceMin struct {
		Image       string `yaml:"image"`
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
	services := make(map[string]serviceInfo, len(index.Services))
	for svc, svcDef := range index.Services {
		hasHealthcheck := len(svcDef.Healthcheck.Test) > 0
		services[svc] = serviceInfo{hasHealthcheck: hasHealthcheck}
	}
	return services, nil
}

func generateServicesOverrideFile(arduinoApp *app.ArduinoApp, services []string, servicesThatRequireDevices []string, devices []string, user string, groups []string, overrideComposeFile *paths.Path) error {
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
		User     string            `yaml:"user,omitempty"`
		Devices  *[]string         `yaml:"devices,omitempty"`
		GroupAdd *[]string         `yaml:"group_add,omitempty"`
		Labels   map[string]string `yaml:"labels,omitempty"`
	}
	var overrideCompose struct {
		Services map[string]serviceOverride `yaml:"services,omitempty"`
	}
	overrideCompose.Services = make(map[string]serviceOverride, len(services))
	for _, svc := range services {
		override := serviceOverride{
			User: user,
			Labels: map[string]string{
				DockerAppLabel:     "true",
				DockerAppPathLabel: arduinoApp.FullPath.String(),
			},
		}
		if slices.Contains(servicesThatRequireDevices, svc) {
			override.Devices = &devices
			override.GroupAdd = &groups
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
