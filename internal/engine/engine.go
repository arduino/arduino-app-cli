// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package engine is what runs apps, seen from the orchestrator: containers, compose
// projects, images. dockerhelper implements it on docker. A nil Engine means there is
// no board: what needs one answers ErrNeedsBoard.
package engine

import (
	"context"
	"errors"
	"io"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/events"
)

// ErrNeedsBoard is the answer of what runs apps where there is no engine.
var ErrNeedsBoard = errors.New("this needs a board")

// ErrNetwork states that the network of the app was not created. A pool with no
// subnet left is one of the reasons, worded by the daemon.
var ErrNetwork = errors.New("failed to create the network of the app")

// ComposeImages lists the images a compose project uses.
func ComposeImages(prj *types.Project) []string {
	images := make([]string, 0, len(prj.Services))
	for _, service := range prj.Services {
		if service.Image != "" && service.Build == nil {
			images = append(images, service.Image)
		}
	}
	return images
}

// BuiltImages names the images the services of the project build, the way compose
// names them.
func BuiltImages(prj *types.Project) []string {
	var images []string
	for name, service := range prj.Services {
		if service.Build == nil {
			continue
		}
		image := service.Image
		if image == "" {
			image = prj.Name + "-" + name
		}
		images = append(images, image)
	}
	return images
}

type Engine interface {
	Version(ctx context.Context) (version, apiVersion string, err error)

	Containers(ctx context.Context, label string) ([]container.Summary, error)
	ContainerEvents(ctx context.Context, label string, actions ...string) (<-chan events.Message, <-chan error)
	StopContainers(ctx context.Context, label string) (int, error)
	PruneContainers(ctx context.Context, label string, keep func(labels map[string]string) bool) (int, error)
	PruneNetworks(ctx context.Context, remove func(labels map[string]string) bool) (int, error)

	ComposeUp(ctx context.Context, prj *types.Project, line func(string)) error
	ComposeStop(ctx context.Context, projectName string, line func(string)) error
	ComposeDown(ctx context.Context, projectName string, line func(string)) error
	ComposeLogs(ctx context.Context, projectName string, opts LogOptions, out LogConsumer) error

	PullImages(ctx context.Context, images []string, line func(string), progress func(label string, curr, total int64)) error
	HasImage(ctx context.Context, image string) (bool, error)
	ListImages(ctx context.Context) ([]string, error)
	RemoveImage(ctx context.Context, image string) (int64, error)

	Run(ctx context.Context, opts RunOptions) error
}

// RunOptions is a container run once, removed when it exits.
type RunOptions struct {
	Image      string
	Entrypoint []string
	Cmd        []string
	Binds      []string
	Env        map[string]string
	Stdout     io.Writer
	Stderr     io.Writer
}

type LogOptions struct {
	Services   []string
	Follow     bool
	Tail       string
	Timestamps bool
}

// LogConsumer takes the lines of a project's containers.
type LogConsumer interface {
	Log(containerName, message string)
	Err(containerName, message string)
	Status(container, msg string)
}
