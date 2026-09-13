// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package dockerhelper

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"
	"github.com/docker/compose/v5/pkg/api"
	"github.com/docker/compose/v5/pkg/compose"
)

// stopTimeout is what a container is given to stop before it is killed.
const stopTimeout = 5 * time.Second

// ComposeUp starts the app the project states, reporting to line what it does. The
// compose of the board takes no part: the sdk this binary links talks to the engine.
func ComposeUp(ctx context.Context, docker command.Cli, prj *types.Project, line func(string)) error {
	// An image the board already has is not asked for again: `--pull missing`.
	for name, service := range prj.Services {
		if service.PullPolicy == "" {
			service.PullPolicy = types.PullPolicyMissing
			prj.Services[name] = service
		}
	}

	stampComposeLabels(prj)

	reported := newProgress(line)
	backend, err := compose.NewComposeService(docker, compose.WithEventProcessor(reported))
	if err != nil {
		return err
	}
	err = backend.Up(ctx, prj, api.UpOptions{
		Create: api.CreateOptions{RemoveOrphans: true, Inherit: true},
		Start:  api.StartOptions{Project: prj},
	})
	// What went wrong is reported as an event, and the error itself says little.
	if err != nil && reported.registryError != nil {
		return reported.registryError
	}
	return err
}

// ComposeStop leaves the containers of the app where they are, stopped.
func ComposeStop(ctx context.Context, docker command.Cli, projectName string, line func(string)) error {
	backend, err := compose.NewComposeService(docker, compose.WithEventProcessor(newProgress(line)))
	if err != nil {
		return err
	}
	timeout := stopTimeout
	return backend.Stop(ctx, projectName, api.StopOptions{Timeout: &timeout})
}

// ComposeDown removes the containers of the app, its volumes and its leftovers.
func ComposeDown(ctx context.Context, docker command.Cli, projectName string, line func(string)) error {
	backend, err := compose.NewComposeService(docker, compose.WithEventProcessor(newProgress(line)))
	if err != nil {
		return err
	}
	timeout := stopTimeout
	return backend.Down(ctx, projectName, api.DownOptions{
		RemoveOrphans: true,
		Volumes:       true,
		Timeout:       &timeout,
	})
}

// stampComposeLabels marks the services the way the sdk marks the projects it loads
// itself: compose finds the containers of a project by these labels.
func stampComposeLabels(prj *types.Project) {
	for name, service := range prj.Services {
		service.CustomLabels = map[string]string{
			api.ProjectLabel:     prj.Name,
			api.ServiceLabel:     name,
			api.VersionLabel:     api.ComposeVersion,
			api.WorkingDirLabel:  prj.WorkingDir,
			api.ConfigFilesLabel: strings.Join(prj.ComposeFiles, ","),
			api.OneoffLabel:      "False",
		}
		prj.Services[name] = service
	}
}

// composeProgress is where the sdk reports to: an api.EventProcessor.
type composeProgress struct {
	line          func(string)
	mu            sync.Mutex
	said          map[string]string
	registryError error
}

func newProgress(line func(string)) *composeProgress {
	if line == nil {
		line = func(string) {}
	}
	return &composeProgress{line: line, said: map[string]string{}}
}

func (p *composeProgress) On(events ...api.Resource) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, event := range events {
		if err := registryError(event.Details); err != nil {
			p.registryError = err
		}
		// The sdk repeats itself for every chunk of every layer: what a resource does
		// is a line, the bytes it is at are not.
		if p.said[event.ID] == event.Text {
			continue
		}
		p.said[event.ID] = event.Text
		if line := strings.TrimSpace(event.ID + " " + event.Text); line != "" {
			p.line(line)
		}
	}
}

// Start and Done say only that an operation began and ended: what the sdk has to say
// is in the resources it reports to On.
func (p *composeProgress) Start(context.Context, string) {}

func (p *composeProgress) Done(string, bool) {}

// registryError states what an event of the registry means: the error of the sdk does not.
func registryError(message string) error {
	if strings.HasSuffix(message, ": unauthorized") {
		return errors.New("could not reach the Docker registry to download base image. Please make sure to be authorized to download from it or flash the board with the latest Arduino Linux image. Details: " + message + ")")
	}

	if strings.HasSuffix(message, ": connection refused") || strings.Contains(message, ": no such host") {
		return errors.New("could not reach the Docker registry to download base image. Please check your internet connection or flash the board with the latest Arduino Linux image. Details: " + message + ")")
	}

	return nil
}
