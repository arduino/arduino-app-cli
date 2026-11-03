// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package dockerhelper is what talks to docker: the engine of the board, through its
// api, and the images of an app, through the registry they come from. Nothing else in
// this project states a container, a network or a compose project.
package dockerhelper

import (
	"context"
	"fmt"
	"io"

	"github.com/arduino/arduino-app-cli/internal/engine"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/flags"
	"github.com/moby/moby/client"
)

// NewCli is the docker client this binary talks to the engine with.
func NewCli() (*command.DockerCli, error) {
	// The endpoint comes from the docker context, as it does for `docker` itself, so a
	// host that keeps its engine elsewhere is reached the same way. It is resolved here
	// and not on the first call, where the cli exits the process on failure.
	opts := flags.NewClientOptions()
	engine, err := command.NewAPIClientFromFlags(opts, config.LoadDefaultConfigFile(io.Discard))
	if err != nil {
		return nil, err
	}
	docker, err := command.NewDockerCli(command.WithAPIClient(engine))
	if err != nil {
		return nil, err
	}
	return docker, docker.Initialize(opts)
}

// EngineVersion is what the engine of this board answers: its version, and the api it
// speaks after the negotiation.
func (e *Engine) Version(ctx context.Context) (version, apiVersion string, err error) {
	docker := e.cli
	v, err := docker.Client().ServerVersion(ctx, client.ServerVersionOptions{})
	if err != nil {
		return "", "", fmt.Errorf("cannot reach the docker engine: %w", err)
	}
	return v.Version, v.APIVersion, nil
}

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

// Close closes the connection to the engine.
func (e *Engine) Close() error { return e.cli.Client().Close() }

// clientOnly is a docker cli made of a client alone, for EngineOfClient.
type clientOnly struct {
	command.Cli
	c client.APIClient
}

func (c clientOnly) Client() client.APIClient { return c.c }

// EngineOfClient wraps a docker client alone, as tests do: what needs the rest of the
// cli, as compose does, panics.
func EngineOfClient(c client.APIClient) *Engine { return &Engine{cli: clientOnly{c: c}} }
