// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build js

package dockerhelper

// The package's api where there is no docker, as in a browser. What reads the engine
// answers as an engine with no container, image or event would: no app runs here, and
// stopping one is already done. What would change the engine answers ErrNoDocker.

import (
	"context"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/events"
)

// NewCli gives the client of a place with no docker.
func NewCli() (Cli, error) { return &NoDocker{}, nil }

func EngineVersion(ctx context.Context, docker Cli) (version, apiVersion string, err error) {
	return "", "", ErrNoDocker
}

func PruneContainers(ctx context.Context, docker EngineClient, label string, keep func(labels map[string]string) bool) (int, error) {
	return 0, nil
}

func PruneNetworks(ctx context.Context, docker EngineClient, remove func(labels map[string]string) bool) (int, error) {
	return 0, nil
}

func Containers(ctx context.Context, docker EngineClient, label string) ([]container.Summary, error) {
	return nil, nil
}

func StopContainers(ctx context.Context, docker EngineClient, label string) (int, error) {
	return 0, nil
}

// ContainerEvents sends no event. Its error channel closes when ctx ends, which is how a
// reader waiting on both learns that the stream is over.
func ContainerEvents(ctx context.Context, docker EngineClient, label string, actions ...string) (<-chan events.Message, <-chan error) {
	errs := make(chan error)
	go func() {
		<-ctx.Done()
		close(errs)
	}()
	return nil, errs
}

func ComposeUp(ctx context.Context, docker Cli, prj *types.Project, line func(string)) error {
	return ErrNoDocker
}

func ComposeStop(ctx context.Context, docker Cli, projectName string, line func(string)) error {
	return nil
}

func ComposeDown(ctx context.Context, docker Cli, projectName string, line func(string)) error {
	return nil
}

func PullImages(ctx context.Context, docker EngineClient, images []string, line func(string), progress func(label string, curr, total int64)) error {
	return ErrNoDocker
}

func HasImage(ctx context.Context, docker EngineClient, image string) (bool, error) {
	return false, nil
}

func ListImages(ctx context.Context, docker EngineClient) ([]string, error) {
	return nil, nil
}

func RemoveImage(ctx context.Context, docker EngineClient, imageName string) (int64, error) {
	return 0, ErrNoDocker
}

func Run(ctx context.Context, docker EngineClient, opts RunOptions) error {
	return ErrNoDocker
}
