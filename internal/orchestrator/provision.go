package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"

	"github.com/arduino/arduino-app-cli/pkg/parser"

	"github.com/arduino/go-paths-helper"
	"github.com/docker/docker/api/types/container"
	dockerClient "github.com/docker/docker/client"
	"github.com/gosimple/slug"
	"go.bug.st/f"
	"gopkg.in/yaml.v3"
)

var containerNameInvalidRegex = regexp.MustCompile(`[^a-zA-Z0-9]`)

func ProvisionApp(ctx context.Context, docker *dockerClient.Client, app parser.App) error {
	if err := pullBasePythonContainer(ctx, pythonImage); err != nil {
		return fmt.Errorf("provisioning failed to pull base image: %w", err)
	}

	resp, err := docker.ContainerCreate(
		ctx,
		&container.Config{
			Image:      pythonImage,
			User:       getCurrentUser(),
			Entrypoint: []string{"/run.sh", "provision"},
		}, &container.HostConfig{
			Binds:      []string{app.FullPath.String() + ":/app"},
			AutoRemove: true,
		},
		nil,
		nil,
		generateContainerName(app.Name))
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

	return generateMainComposeFile(ctx, app, pythonImage)
}

// Converts an arbitrary string to one that satisfies the container name requirement: [a-zA-Z0-9][a-zA-Z0-9_.-]
// See the Docker Engine code here: https://github.com/moby/moby/blob/master/daemon/names/names.go#L6
func generateContainerName(appName string) string {
	result := containerNameInvalidRegex.ReplaceAllString(appName, "")

	if len(result) < 1 {
		result = "c"
	}
	return result
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

func getProvisioningStateDir(app parser.App) (*paths.Path, error) {
	cacheDir := app.FullPath.Join(".cache")
	if err := cacheDir.MkdirAll(); err != nil {
		return nil, err
	}
	return cacheDir, nil
}

const DockerAppLabel = "cc.arduino.app"
const DockerAppPathLabel = "cc.arduino.app.path"

func generateMainComposeFile(ctx context.Context, app parser.App, pythonImage string) error {
	provisioningStateDir, err := getProvisioningStateDir(app)
	if err != nil {
		return err
	}

	var composeFiles paths.PathList
	for _, brick := range app.Descriptor.Bricks {
		composeFilePath := provisioningStateDir.Join("compose", brick.Name, "module_compose.yaml")
		if composeFilePath.Exist() {
			composeFiles.Add(composeFilePath)
			slog.Debug("Brick compose file found", slog.String("module", brick.Name), slog.String("path", composeFilePath.String()))
		} else {
			slog.Debug("Brick compose file not found", slog.String("module", brick.Name), slog.String("path", composeFilePath.String()))
		}
	}

	// Create a single docker-mainCompose that includes all the required services
	mainComposeFile := provisioningStateDir.Join("app-compose.yaml")

	type service struct {
		Image      string            `yaml:"image"`
		DependsOn  []string          `yaml:"depends_on,omitempty"`
		Volumes    []string          `yaml:"volumes"`
		Devices    []string          `yaml:"devices"`
		Ports      []string          `yaml:"ports"`
		User       string            `yaml:"user"`
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
	mainAppCompose.Include = composeFiles.AsStrings()

	composeProjectName, err := app.FullPath.RelFrom(orchestratorConfig.AppsDir())
	if err != nil {
		return fmt.Errorf("failed to get compose project name: %w", err)
	}
	mainAppCompose.Name = slug.Make(composeProjectName.String())
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

	ports := make([]string, len(app.Descriptor.Ports))
	for i, p := range app.Descriptor.Ports {
		ports[i] = fmt.Sprintf("%d:%d", p, p)
	}

	mainAppCompose.Services = &mainService{
		Main: service{
			Image:      pythonImage,
			Volumes:    []string{app.FullPath.String() + ":/app"},
			Ports:      ports,
			Devices:    getDevices(),
			Entrypoint: "/run.sh",
			DependsOn:  services,
			User:       getCurrentUser(),
			ExtraHosts: []string{"msgpack-rpc-router:host-gateway"},
			Labels: map[string]string{
				DockerAppLabel:     "true",
				DockerAppPathLabel: app.FullPath.String(),
			},
		},
	}
	return writeMainCompose()
}
