package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/pkg/parser"
)

func dockerComposeListServices(ctx context.Context, composeFile *paths.Path) ([]string, error) {
	process, err := paths.NewProcess(nil, "docker", "compose", "-f", composeFile.String(), "config", "--services")
	if err != nil {
		return nil, err
	}
	stdout, stderr, err := process.RunAndCaptureOutput(ctx)
	if len(stderr) > 0 {
		slog.Error("docker compose config error", slog.String("stderr", string(stderr)))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to run docker compose config: %w", err)
	}

	if len(stdout) == 0 {
		return nil, nil
	}

	return strings.Split(strings.TrimSpace(string(stdout)), "\n"), nil
}

type DockerComposeAppStatusResponse struct {
	Name   string `json:"Name"`
	Status string `json:"Status"`
}

func dockerComposeAppStatus(ctx context.Context, app parser.App) (DockerComposeAppStatusResponse, error) {
	mainCompose, err := getProvisioningStateDir(app)
	if err != nil {
		return DockerComposeAppStatusResponse{}, err
	}
	composeName := app.FullPath.Base()

	process, err := paths.NewProcess(nil, "docker", "compose", "-f", mainCompose.String(), "ls", "--format", "json", "--all", "--filter", fmt.Sprintf("name=%s", composeName))
	if err != nil {
		return DockerComposeAppStatusResponse{}, err
	}
	stdout, stderr, err := process.RunAndCaptureOutput(ctx)
	if len(stderr) > 0 {
		slog.Error("docker compose config error", slog.String("stderr", string(stderr)))
	}
	if err != nil {
		return DockerComposeAppStatusResponse{}, fmt.Errorf("failed to run docker compose config: %w", err)
	}

	var statusResponse []DockerComposeAppStatusResponse
	if err := json.Unmarshal(stdout, &statusResponse); err != nil {
		return DockerComposeAppStatusResponse{}, fmt.Errorf("failed to unmarshal docker compose status response: %w", err)
	}

	if len(statusResponse) == 0 {
		return DockerComposeAppStatusResponse{}, fmt.Errorf("failed to find app status in docker compose response")
	}
	// We only want the first response, as we are filtering by name
	resp := statusResponse[0]

	// The response from compose is in the form of "state(number_services)". Example: "running(2)"
	// We only want the state, so we remove the number of services
	idx := strings.Index(resp.Status, "(")
	if idx != -1 {
		resp.Status = resp.Status[:idx]
	}

	return resp, nil
}
