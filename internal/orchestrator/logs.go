// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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

	"github.com/arduino/arduino-app-cli/internal/helpers"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
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
	bricksIndex *bricksindex.BricksIndex,
) (iter.Seq[LogMessage], error) {
	if app.MainPythonFile == nil {
		return helpers.EmptyIter[LogMessage](), nil
	}

	mainCompose := app.AppComposeFilePath()
	if mainCompose.NotExist() {
		return helpers.EmptyIter[LogMessage](), nil
	}

	bricksIndex = bricksIndex.WithAppBricks(app.LocalBricks)

	// Obtain mapping compose service name <-> brick name
	serviceToBrickMapping := make(map[string]string, len(app.Descriptor.Bricks))
	for _, brick := range app.Descriptor.Bricks {
		brick, ok := bricksIndex.FindBrickByID(brick.ID)
		if !ok {
			slog.Warn("brick not valid", slog.String("brick_id", brick.ID))
			continue
		}
		composeFilePath, found := brick.GetComposeFile()
		if !found {
			slog.Warn("brick compose id not valid", slog.String("brick_id", brick.ID))
			continue
		}
		if !composeFilePath.Exist() {
			slog.Debug("Brick compose file not found", slog.String("module", brick.ID), slog.String("path", composeFilePath.String()))
			continue
		}

		services, err := extractServicesFromComposeFile(composeFilePath)
		if err != nil {
			return helpers.EmptyIter[LogMessage](), err
		}
		for _, s := range services {
			serviceToBrickMapping[s.name] = brick.ID
		}
	}

	prj, err := loader.LoadWithContext(
		ctx,
		types.ConfigDetails{
			ConfigFiles: []types.ConfigFile{{Filename: mainCompose.String()}},
			WorkingDir:  app.ProvisioningStateDir().String(),
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
