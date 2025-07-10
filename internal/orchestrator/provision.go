package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/assets"

	"github.com/arduino/go-paths-helper"
	"github.com/docker/docker/api/types/container"
	dockerClient "github.com/docker/docker/client"
	yaml "github.com/goccy/go-yaml"
	"go.bug.st/f"
)

func ProvisionApp(ctx context.Context, docker *dockerClient.Client, app app.ArduinoApp) error {
	start := time.Now()
	defer func() {
		slog.Info("Provisioning took", "duration", time.Since(start).String())
	}()

	var containsThirdPartyDeps bool
	for _, dep := range app.Descriptor.Bricks {
		if !strings.HasPrefix(dep.ID, "arduino:") {
			containsThirdPartyDeps = true
			break
		}
	}
	// In case in local development we use a tag that is not in the index we
	// fallback to dynamicProvisioning
	if containsThirdPartyDeps || usedPythonImageTag != bricksVersion.String() {
		if err := dynamicProvisioning(ctx, docker, app); err != nil {
			return err
		}
	} else {
		cFS, err := fs.Sub(assets.FS, path.Join("static", bricksVersion.String()))
		if err != nil {
			return fmt.Errorf("failed to read assets directory: %w", err)
		}

		provisioningStateDir, err := getProvisioningStateDir(app)
		if err != nil {
			return err
		}
		if err := os.CopyFS(provisioningStateDir.String(), cFS); err != nil {
			if errors.Is(err, fs.ErrExist) {
				if err := provisioningStateDir.Join("models-list.yaml").Remove(); err != nil {
					return err
				}
				if err := provisioningStateDir.Join("bricks-list.yaml").Remove(); err != nil {
					return err
				}
				if err := provisioningStateDir.Join("compose").RemoveAll(); err != nil {
					return err
				}
				if err := provisioningStateDir.Join("docs").RemoveAll(); err != nil {
					return err
				}

				// try again
				if err := os.CopyFS(provisioningStateDir.String(), cFS); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("failed to copy assets directory: %w", err)
			}
		}
	}

	return generateMainComposeFile(ctx, app, pythonImage)
}

func dynamicProvisioning(ctx context.Context, docker *dockerClient.Client, app app.ArduinoApp) error {
	if err := pullBasePythonContainer(ctx, pythonImage); err != nil {
		return fmt.Errorf("provisioning failed to pull base image: %w", err)
	}

	containerCfg := &container.Config{
		Image:      pythonImage,
		User:       getCurrentUser(),
		Entrypoint: []string{"/run.sh", "provision"},
	}
	containerHostCfg := &container.HostConfig{
		Binds:      []string{app.FullPath.String() + ":/app"},
		AutoRemove: true,
	}
	resp, err := docker.ContainerCreate(ctx, containerCfg, containerHostCfg, nil, nil, "")
	if err != nil {
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

func generateMainComposeFile(ctx context.Context, app app.ArduinoApp, pythonImage string) error {
	provisioningStateDir, err := getProvisioningStateDir(app)
	if err != nil {
		return err
	}

	var composeFiles paths.PathList
	for _, brick := range app.Descriptor.Bricks {
		brickPath := filepath.Join(strings.Split(brick.ID, ":")...)
		composeFilePath := provisioningStateDir.Join("compose", brickPath, "brick_compose.yaml")
		if composeFilePath.Exist() {
			composeFiles.Add(composeFilePath)
			slog.Debug("Brick compose file found", slog.String("module", brick.ID), slog.String("path", composeFilePath.String()))
		} else {
			slog.Debug("Brick compose file not found", slog.String("module", brick.ID), slog.String("path", composeFilePath.String()))
		}
	}

	// Create a single docker-mainCompose that includes all the required services
	mainComposeFile := provisioningStateDir.Join("app-compose.yaml")

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
	composeProjectName, err := getAppComposeProjectNameFromApp(app)
	if err != nil {
		return err
	}
	mainAppCompose.Name = composeProjectName
	mainAppCompose.Include = composeFiles.AsStrings()
	if err := writeMainCompose(); err != nil {
		return err
	}

	slog.Debug("Compose file for the App created", slog.String("compose_file", mainComposeFile.String()))

	// docker compose -f app-compose.yml config --services
	services, err := dockerComposeListServices(ctx, mainComposeFile)
	if err != nil {
		return err
	}
	services = f.Filter(services, f.NotEquals("main"))

	ports := make(map[string]struct{}, len(app.Descriptor.Ports))
	for _, p := range app.Descriptor.Ports {
		ports[fmt.Sprintf("%d:%d", p, p)] = struct{}{}
	}

	for _, b := range app.Descriptor.Bricks {
		brick, found := bricksIndex.FindBrickByID(b.ID)
		if !found {
			continue
		}
		for _, p := range brick.Ports {
			ports[fmt.Sprintf("%s:%s", p, p)] = struct{}{}
		}
	}

	volumes := []volume{
		{
			Type:   "bind",
			Source: app.FullPath.String(),
			Target: "/app",
		},
	}
	if orchestratorConfig.RouterSocketPath().Exist() {
		volumes = append(volumes, volume{
			Type:   "bind",
			Source: orchestratorConfig.RouterSocketPath().String(),
			Target: "/var/run/arduino-router.sock",
		})
	}

	mainAppCompose.Services = &mainService{
		Main: service{
			Image:      pythonImage,
			Volumes:    volumes,
			Ports:      slices.Collect(maps.Keys(ports)),
			Devices:    getDevices(),
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
	return writeMainCompose()
}
