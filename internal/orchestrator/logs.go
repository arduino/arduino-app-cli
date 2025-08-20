package orchestrator

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"
	commands "github.com/docker/compose/v2/cmd/compose"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/docker/compose/v2/pkg/compose"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/pkg/x"
)

type AppLogsRequest struct {
	ShowAppLogs      bool
	ShowServicesLogs bool
	Follow           bool
	Tail             *uint64
}

type LogMessage struct {
	Name      string
	BrickName string
	Content   string
}

func AppLogs(
	ctx context.Context,
	app app.ArduinoApp,
	req AppLogsRequest,
	dockerCli command.Cli,
) (iter.Seq[LogMessage], error) {
	if app.MainPythonFile == nil {
		return x.EmptyIter[LogMessage](), nil
	}

	mainCompose := app.AppComposeFilePath()
	if mainCompose.NotExist() {
		return x.EmptyIter[LogMessage](), nil
	}

	configFiles := []types.ConfigFile{{Filename: mainCompose.String()}}
	// Obtain mapping compose service name <-> brick name
	serviceToBrickMapping := make(map[string]string, len(app.Descriptor.Bricks))
	for _, brick := range app.Descriptor.Bricks {
		namespace, brickName, ok := strings.Cut(brick.ID, ":")
		if !ok {
			slog.Warn("invalid brick id", slog.String("brick_id", brick.ID))
			continue
		}
		composeFilePath := app.ProvisioningStateDir().Join("compose", namespace, brickName, "brick_compose.yaml")
		if composeFilePath.Exist() {
			prj, err := loader.LoadWithContext(
				ctx,
				types.ConfigDetails{
					ConfigFiles: []types.ConfigFile{{Filename: composeFilePath.String()}},
					Environment: types.NewMapping(os.Environ()),
				},
				// This is used otherwise compose will fail with: project name must be set
				func(o *loader.Options) { o.SetProjectName(brick.ID, false) },
			)
			if err != nil {
				return x.EmptyIter[LogMessage](), err
			}
			for s := range prj.Services {
				serviceToBrickMapping[s] = brick.ID
			}
			configFiles = append(configFiles, types.ConfigFile{Filename: composeFilePath.String()})
			slog.Debug("Brick compose file found", slog.String("module", brick.ID), slog.String("path", composeFilePath.String()))
		} else {
			slog.Debug("Brick compose file not found", slog.String("module", brick.ID), slog.String("path", composeFilePath.String()))
		}
	}

	prj, err := loader.LoadWithContext(
		ctx,
		types.ConfigDetails{
			ConfigFiles: configFiles,
			WorkingDir:  mainCompose.Base(),
			Environment: types.NewMapping(os.Environ()),
		},
		loader.WithSkipValidation, //TODO: check if there is a bug on docker compose upstream
	)
	if err != nil {
		return nil, err
	}

	filteredServices := prj.ServiceNames()
	if req.ShowAppLogs && !req.ShowServicesLogs {
		filteredServices = []string{"main"}
	} else if req.ShowServicesLogs && !req.ShowAppLogs {
		filteredServices = f.Filter(filteredServices, f.NotEquals("main"))
	}

	backend := compose.NewComposeService(dockerCli).(commands.Backend)
	return func(yield func(LogMessage) bool) {
		opts := api.LogOptions{
			Project:    prj,
			Follow:     req.Follow,
			Services:   filteredServices,
			Timestamps: false,
		}
		if req.Tail != nil {
			opts.Tail = fmt.Sprintf("%d", *req.Tail)
		}
		err = backend.Logs(
			ctx,
			prj.Name,
			NewDockerLogConsumer(ctx, yield, serviceToBrickMapping),
			opts,
		)
		if err != nil {
			slog.Error("docker logs error", slog.String("error", err.Error()))
			return
		}
	}, nil
}

var _ api.LogConsumer = (*DockerLogConsumer)(nil)

type DockerLogConsumer struct {
	ctx          context.Context
	cb           func(LogMessage) bool
	mapping      map[string]string
	shuttingDown atomic.Bool
	mu           sync.Mutex
}

func NewDockerLogConsumer(
	ctx context.Context,
	cb func(LogMessage) bool,
	mapping map[string]string,
) *DockerLogConsumer {
	return &DockerLogConsumer{
		ctx:     ctx,
		cb:      cb,
		mapping: mapping,
	}
}

// Err implements api.LogConsumer.
func (d *DockerLogConsumer) Err(containerName string, message string) {
	d.write(containerName, message)
}

// Log implements api.LogConsumer.
func (d *DockerLogConsumer) Log(containerName string, message string) {
	d.write(containerName, message)
}

// Status implements api.LogConsumer.
func (d *DockerLogConsumer) Status(container string, msg string) {
	d.write(container, msg)
}

func (d *DockerLogConsumer) write(container, message string) {
	if d.ctx.Err() != nil || d.shuttingDown.Load() {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.shuttingDown.Load() {
		return
	}

	serviceName := strings.TrimSpace(container)
	idx := strings.LastIndex(serviceName, "-")
	if idx != -1 {
		// remove the suffix -1 or -2 or -4
		serviceName = serviceName[:idx]
	}
	for line := range strings.SplitSeq(message, "\n") {
		if !d.cb(LogMessage{
			Name:      serviceName,
			BrickName: d.mapping[serviceName],
			Content:   line,
		}) {
			d.shuttingDown.CompareAndSwap(false, true)
			return
		}
	}
}
