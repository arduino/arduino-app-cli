package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/containerd/errdefs"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"

	"github.com/arduino/go-paths-helper"
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
	app *app.ArduinoApp,
) error {
	start := time.Now()
	defer func() {
		slog.Info("Provisioning took", "duration", time.Since(start).String())
	}()
	return provisioner.App(ctx, bricksIndex, app)
}

type Provision struct {
	docker              *dockerClient.Client
	useDynamicProvision bool
	composeFS           fs.FS
	pythonImage         string
}

func NewProvision(
	docker *dockerClient.Client,
	assetsFS fs.FS,
	useDynamicProvision bool,
	pythonImage string,
) (*Provision, error) {
	if useDynamicProvision {
		if err := dynamicProvisioning(context.Background(), docker, pythonImage, paths.TempDir().String()); err != nil {
			return nil, fmt.Errorf("failed to perform dynamic provisioning: %w", err)
		}
	}

	return &Provision{
		docker:              docker,
		composeFS:           assetsFS,
		useDynamicProvision: useDynamicProvision,
		pythonImage:         pythonImage,
	}, nil
}

func (p *Provision) App(
	ctx context.Context,
	bricksIndex *bricksindex.BricksIndex,
	arduinoApp *app.ArduinoApp,
) error {
	if arduinoApp == nil {
		return fmt.Errorf("provisioning failed: arduinoApp is nil")
	}

	provisioningStateDir, err := getProvisioningStateDir(*arduinoApp)
	if err != nil {
		return err
	}

	dst := provisioningStateDir.Join("compose")
	if err := dst.RemoveAll(); err != nil {
		return fmt.Errorf("failed to remove compose directory: %w", err)
	}
	if p.useDynamicProvision {
		composeDir := paths.New(paths.TempDir().String(), ".cache", "compose")
		if err := composeDir.CopyDirTo(dst); err != nil {
			return fmt.Errorf("failed to copy compose directory: %w", err)
		}
	} else {
		if err := os.CopyFS(dst.String(), p.composeFS); err != nil {
			return fmt.Errorf("failed to copy assets directory: %w", err)
		}
	}

	return generateMainComposeFile(arduinoApp, bricksIndex, p.pythonImage)
}

func dynamicProvisioning(
	ctx context.Context,
	docker *dockerClient.Client,
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
		}
		return fmt.Errorf("provisiong failed to create container: %w", err)
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
	process.RedirectStdoutTo(os.Stdout)
	process.RedirectStderrTo(os.Stderr)
	return process.RunWithinContext(ctx)
}

func getProvisioningStateDir(app app.ArduinoApp) (*paths.Path, error) {
	cacheDir := app.FullPath.Join(".cache")
	if err := cacheDir.MkdirAll(); err != nil {
		return nil, err
	}
	return cacheDir, nil
}

const DockerAppLabel = "cc.arduino.app"
const DockerAppPathLabel = "cc.arduino.app.path"

func generateMainComposeFile(
	app *app.ArduinoApp,
	bricksIndex *bricksindex.BricksIndex,
	pythonImage string,
) error {
	provisioningStateDir, err := getProvisioningStateDir(*app)
	if err != nil {
		return err
	}

	slog.Debug("Generating main compose file for the App")

	var composeFiles paths.PathList
	services := []string{}
	brickServices := map[string][]string{}
	for _, brick := range app.Descriptor.Bricks {
		brickPath := filepath.Join(strings.Split(brick.ID, ":")...)
		composeFilePath := provisioningStateDir.Join("compose", brickPath, "brick_compose.yaml")
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
	mainComposeFile := provisioningStateDir.Join("app-compose.yaml")
	// If required, create an override compose file for devices
	overrideComposeFile := provisioningStateDir.Join("app-compose-overrides.yaml")

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

	mainAppCompose.Services = &mainService{
		Main: service{
			Image:      pythonImage,
			Volumes:    volumes,
			Ports:      slices.Collect(maps.Keys(ports)),
			Devices:    devices,
			Entrypoint: "/run.sh",
			DependsOn:  services,
			User:       getCurrentUser(),
			GroupAdd:   []string{"dialout", "video", "audio"},
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
	if overrideComposeFile.Exist() {
		if err := overrideComposeFile.Remove(); err != nil {
			return fmt.Errorf("failed to remove existing override compose file: %w", err)
		}
	}
	if len(servicesThatRequireDevices) > 0 {
		// Write additiona file to override devices section in included compose files
		if e := generateServicesOverrideFile(servicesThatRequireDevices, devices, overrideComposeFile); e != nil {
			return e
		}
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

func generateServicesOverrideFile(servicesThatRequireDevices []string, devices []string, overrideComposeFile *paths.Path) error {
	type serviceOverride struct {
		Devices []string `yaml:"devices"`
	}
	var overrideCompose struct {
		Services map[string]serviceOverride `yaml:"services,omitempty"`
	}
	overrideCompose.Services = make(map[string]serviceOverride, len(servicesThatRequireDevices))
	for _, svc := range servicesThatRequireDevices {
		overrideCompose.Services[svc] = serviceOverride{
			Devices: devices,
		}
	}
	slog.Debug("Generating override compose file for devices", slog.Any("overrideCompose", overrideCompose), slog.Any("devices", devices))
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
