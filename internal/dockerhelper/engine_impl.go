// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package dockerhelper

import (
	"context"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"
	"github.com/docker/compose/v5/pkg/api"
	"github.com/docker/compose/v5/pkg/compose"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client"

	"github.com/arduino/arduino-app-cli/internal/engine"
)

// Engine is engine.Engine on docker.
type Engine struct{ cli command.Cli }

var _ engine.Engine = (*Engine)(nil)

// NewEngine connects to the docker engine as NewCli does.
func NewEngine() (*Engine, error) {
	cli, err := NewCli()
	if err != nil {
		return nil, err
	}
	return &Engine{cli: cli}, nil
}

// EngineOf wraps a docker cli already made, as tests do.
func EngineOf(cli command.Cli) *Engine { return &Engine{cli: cli} }

func (e *Engine) Close() error { return e.cli.Client().Close() }

func (e *Engine) Version(ctx context.Context) (string, string, error) {
	return EngineVersion(ctx, e.cli)
}

func (e *Engine) Containers(ctx context.Context, label string) ([]container.Summary, error) {
	return Containers(ctx, e.cli.Client(), label)
}

func (e *Engine) ContainerEvents(ctx context.Context, label string, actions ...string) (<-chan events.Message, <-chan error) {
	return ContainerEvents(ctx, e.cli.Client(), label, actions...)
}

func (e *Engine) StopContainers(ctx context.Context, label string) (int, error) {
	return StopContainers(ctx, e.cli.Client(), label)
}

func (e *Engine) PruneContainers(ctx context.Context, label string, keep func(map[string]string) bool) (int, error) {
	return PruneContainers(ctx, e.cli.Client(), label, keep)
}

func (e *Engine) PruneNetworks(ctx context.Context, remove func(map[string]string) bool) (int, error) {
	return PruneNetworks(ctx, e.cli.Client(), remove)
}

func (e *Engine) ComposeUp(ctx context.Context, prj *types.Project, line func(string)) error {
	return ComposeUp(ctx, e.cli, prj, line)
}

func (e *Engine) ComposeStop(ctx context.Context, projectName string, line func(string)) error {
	return ComposeStop(ctx, e.cli, projectName, line)
}

func (e *Engine) ComposeDown(ctx context.Context, projectName string, line func(string)) error {
	return ComposeDown(ctx, e.cli, projectName, line)
}

func (e *Engine) ComposeLogs(ctx context.Context, projectName string, opts engine.LogOptions, out engine.LogConsumer) error {
	backend, err := compose.NewComposeService(e.cli)
	if err != nil {
		return err
	}
	return backend.Logs(ctx, projectName, out, api.LogOptions{
		Services:   opts.Services,
		Follow:     opts.Follow,
		Tail:       opts.Tail,
		Timestamps: opts.Timestamps,
	})
}

func (e *Engine) PullImages(ctx context.Context, images []string, line func(string), progress func(string, int64, int64)) error {
	return PullImages(ctx, e.cli.Client(), images, line, progress)
}

func (e *Engine) HasImage(ctx context.Context, image string) (bool, error) {
	return HasImage(ctx, e.cli.Client(), image)
}

func (e *Engine) ListImages(ctx context.Context) ([]string, error) {
	return ListImages(ctx, e.cli.Client())
}

func (e *Engine) RemoveImage(ctx context.Context, image string) (int64, error) {
	return RemoveImage(ctx, e.cli.Client(), image)
}

func (e *Engine) Run(ctx context.Context, opts engine.RunOptions) error {
	return Run(ctx, e.cli.Client(), RunOptions(opts))
}

// clientOnly is a docker cli made of a client alone, for EngineOfClient.
type clientOnly struct {
	command.Cli
	c client.APIClient
}

func (c clientOnly) Client() client.APIClient { return c.c }

// EngineOfClient wraps a docker client alone, as tests do: what needs the rest of the
// cli, as compose does, panics.
func EngineOfClient(c client.APIClient) *Engine { return &Engine{cli: clientOnly{c: c}} }
