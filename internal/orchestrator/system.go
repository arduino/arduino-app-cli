package orchestrator

import (
	"context"
	"os"

	"github.com/arduino/go-paths-helper"
)

// SystemInit pulls necessary Docker images.
func SystemInit(ctx context.Context) error {
	preInstallContainer := []string{
		"ghcr.io/bcmi-labs/arduino/appslab-python-apps-base:" + usedPythonImageTag,
		"ghcr.io/bcmi-labs/arduino/appslab-ei-models-runner:" + usedPythonImageTag,
	}

	args := make([]string, 0, 3)
	for _, container := range preInstallContainer {
		args = args[:0] // Reset args for each container
		args = append(args, "docker", "pull", container)
		cmd, err := paths.NewProcess(nil, args...)
		if err != nil {
			return err
		}
		cmd.RedirectStderrTo(os.Stdout)
		cmd.RedirectStdoutTo(os.Stdout)
		if err := cmd.RunWithinContext(ctx); err != nil {
			return err
		}
	}
	return nil
}
