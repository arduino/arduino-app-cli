// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

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
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
)

type AppLogsRequest struct {
	ShowAppLogs      bool
	ShowServicesLogs bool
	Follow           bool
	Tail             *uint64
}

type LogSource string

const (
	LogSourceMain  LogSource = "main"
	LogSourceBrick LogSource = "brick"
)

type LogMessage struct {
	Source        LogSource
	BrickID       string // empty when Source != LogSourceBrick
	ContainerName string
	Content       string
}

func AppLogs(
	ctx context.Context,
	app app.ArduinoApp,
	req AppLogsRequest,
	dockerCli command.Cli,
	bricksIndex *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
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
	for _, appBrick := range app.Descriptor.Bricks {
		brick, ok := bricksIndex.FindBrickByID(appBrick.ID)
		if !ok {
			slog.Warn("brick not valid", slog.String("brick_id", appBrick.ID))
			continue
		}
		composeFilePath, found := brick.GetComposeFile()
		if found && composeFilePath.Exist() {
			services, err := extractServicesFromComposeFile(composeFilePath)
			if err != nil {
				return helpers.EmptyIter[LogMessage](), err
			}
			for _, s := range services {
				serviceToBrickMapping[s.name] = brick.ID
			}
		}

		// Also attribute containers of Arduino Services required by this brick.
		requiredServices, err := brick.GetMatchingService(appBrick.Model)
		if err != nil {
			slog.Warn("failed to get required services for brick", slog.String("brick_id", brick.ID), slog.Any("error", err))
			continue
		}
		for _, serviceID := range requiredServices {
			slog.Debug("brick requires service", slog.String("brick_id", brick.ID), slog.String("service_id", serviceID))
			service, found := servicesIndex.FindServiceByID(serviceID)
			if !found {
				continue
			}
			serviceCompose, ok := service.GetComposeFile()
			if !ok {
				continue
			}
			slog.Debug("loading service compose", slog.String("service_id", serviceID), slog.String("path", serviceCompose.String()))
			services, err := extractServicesFromComposeFile(serviceCompose)
			if err != nil {
				slog.Warn("failed to load service compose", slog.String("service_id", serviceID), slog.Any("error", err))
				continue
			}
			for _, s := range services {
				if _, exists := serviceToBrickMapping[s.name]; !exists {
					serviceToBrickMapping[s.name] = brick.ID
				}
			}
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

	msg := LogMessage{Source: LogSourceMain, ContainerName: serviceName}
	if brickID, ok := d.mapping[serviceName]; ok {
		msg.Source = LogSourceBrick
		msg.BrickID = brickID
	}
	for line := range strings.SplitSeq(message, "\n") {
		msg.Content = line
		if !d.cb(msg) {
			d.shuttingDown.CompareAndSwap(false, true)
			return
		}
	}
}
