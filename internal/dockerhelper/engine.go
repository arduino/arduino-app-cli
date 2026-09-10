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

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/moby/moby/client"
)

// NewCli is the docker client this binary talks to the engine with.
func NewCli() (*command.DockerCli, error) {
	engine, err := client.New(client.FromEnv)
	if err != nil {
		return nil, err
	}
	docker, err := command.NewDockerCli(command.WithAPIClient(engine))
	if err != nil {
		return nil, err
	}
	return docker, docker.Initialize(flags.NewClientOptions())
}

// EngineVersion is what the engine of this board answers: its version, and the api it
// speaks after the negotiation.
func EngineVersion(ctx context.Context, docker command.Cli) (version, apiVersion string, err error) {
	engine, err := docker.Client().ServerVersion(ctx, client.ServerVersionOptions{})
	if err != nil {
		return "", "", fmt.Errorf("cannot reach the docker engine: %w", err)
	}
	return engine.Version, engine.APIVersion, nil
}
