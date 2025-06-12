package orchestrator

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"strings"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/pkg/x"

	"github.com/arduino/go-paths-helper"
	"go.bug.st/f"
)

type AppLogsRequest struct {
	ShowAppLogs      bool
	ShowServicesLogs bool
	Follow           bool
	Tail             int64
}

type LogMessage struct {
	Name      string
	BrickName string
	Content   string
}

func AppLogs(ctx context.Context, app app.ArduinoApp, req AppLogsRequest) (iter.Seq[LogMessage], error) {
	if app.MainPythonFile == nil {
		return x.EmptyIter[LogMessage](), nil
	}

	provisioningStateDir, err := getProvisioningStateDir(app)
	if err != nil {
		return nil, err
	}

	// Obtain mapping compose service name <-> brick name
	serviceToBrickMapping := make(map[string]string, len(app.Descriptor.Bricks))
	for _, brick := range app.Descriptor.Bricks {
		composeFilePath := provisioningStateDir.Join("compose", brick.Name, "brick_compose.yaml")
		if composeFilePath.Exist() {
			services, err := dockerComposeListServices(ctx, composeFilePath)
			if err != nil {
				return x.EmptyIter[LogMessage](), err
			}
			for i := range services {
				serviceToBrickMapping[services[i]] = brick.Name
			}
			slog.Debug("Brick compose file found", slog.String("module", brick.Name), slog.String("path", composeFilePath.String()))
		} else {
			slog.Debug("Brick compose file not found", slog.String("module", brick.Name), slog.String("path", composeFilePath.String()))
		}
	}

	mainCompose := provisioningStateDir.Join("app-compose.yaml")

	dockerComposeServices, err := dockerComposeListServices(ctx, mainCompose)
	if err != nil {
		return nil, err
	}

	if req.ShowAppLogs && !req.ShowServicesLogs {
		dockerComposeServices = []string{"main"}
	} else if req.ShowServicesLogs && !req.ShowAppLogs {
		dockerComposeServices = f.Filter(dockerComposeServices, f.NotEquals("main"))
	}

	args := []string{
		"docker",
		"compose",
		"-f",
		mainCompose.String(),
		"logs",
		"main",
		"--no-color",
	}
	if req.Follow {
		args = append(args, "--follow")
	}
	if req.Tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", req.Tail))
	}
	args = append(args, dockerComposeServices...)
	process, err := paths.NewProcess(nil, args...)
	if err != nil {
		return nil, err
	}
	return func(yield func(LogMessage) bool) {
		stdout := NewCallbackWriter(func(line string) {
			if !yield(convertDockerLogToLogMessage(line, serviceToBrickMapping)) {
				return
			}
		})
		process.RedirectStdoutTo(stdout)

		if err := process.RunWithinContext(ctx); err != nil {
			return
		}
	}, nil
}

func convertDockerLogToLogMessage(m string, serviceToBrickMapping map[string]string) LogMessage {
	serviceName, content, found := strings.Cut(m, "|")
	if !found {
		return LogMessage{Content: m}
	}

	serviceName = strings.TrimSpace(serviceName)
	idx := strings.LastIndex(serviceName, "-")
	if idx != -1 {
		// remove the suffix -1 or -2 or -4
		serviceName = serviceName[:idx]
	}
	return LogMessage{
		Name:      serviceName,
		BrickName: serviceToBrickMapping[serviceName],
		Content:   strings.TrimSpace(content),
	}
}
