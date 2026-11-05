// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package dockerhelper

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/streams"
	"github.com/docker/compose/v5/pkg/api"
	"github.com/docker/compose/v5/pkg/compose"
	dockerClient "github.com/moby/moby/client"
)

// stopTimeout is what a container is given to stop before it is killed.
const stopTimeout = 5 * time.Second

// ComposeImages is every image the project names, which is what a pull is given. An
// image a service builds is not: the registry does not have it.
func ComposeImages(prj *types.Project) []string {
	images := make([]string, 0, len(prj.Services))
	for _, service := range prj.Services {
		if service.Image != "" && service.Build == nil {
			images = append(images, service.Image)
		}
	}
	return images
}

// ComposeUp starts the app the project states, reporting to line what it does. The
// compose of the board takes no part: the sdk this binary links talks to the engine.
func ComposeUp(ctx context.Context, docker command.Cli, prj *types.Project, line func(string)) error {
	// An image the board already has is not asked for again: `--pull missing`. A built
	// one is built at every start, and the cache keeps an unchanged one as it is.
	for name, service := range prj.Services {
		if service.PullPolicy == "" {
			service.PullPolicy = types.PullPolicyMissing
			if service.Build != nil {
				service.PullPolicy = types.PullPolicyBuild
			}
			prj.Services[name] = service
		}
	}

	stampComposeLabels(prj)

	reported := newProgress(line)
	builder := classicBuilder{Cli: docker, out: streams.NewOut(reported)}
	backend, err := compose.NewComposeService(builder, compose.WithEventProcessor(reported))
	if err != nil {
		return err
	}
	err = backend.Up(ctx, prj, api.UpOptions{
		Create: api.CreateOptions{RemoveOrphans: true, Inherit: true, Build: &api.BuildOptions{}},
		Start:  api.StartOptions{Project: prj},
	})
	// A prune never removes an image a container uses, so a failed up prunes too.
	pruneDanglingBuilds(ctx, docker, prj.Name)
	// What went wrong is reported as an event, and the error itself says little.
	if err != nil && reported.registryError != nil {
		return reported.registryError
	}
	if err != nil && reported.networkError != nil {
		return fmt.Errorf("%w: %w", ErrNetwork, reported.networkError)
	}
	return err
}

// ErrNetwork states that the network of the app was not created. A pool with no
// subnet left is one of the reasons, worded by the daemon.
var ErrNetwork = errors.New("failed to create the network of the app")

// ComposeStop leaves the containers of the app where they are, stopped.
func ComposeStop(ctx context.Context, docker command.Cli, projectName string, line func(string)) error {
	backend, err := compose.NewComposeService(docker, compose.WithEventProcessor(newProgress(line)))
	if err != nil {
		return err
	}
	timeout := stopTimeout
	if err := backend.Stop(ctx, projectName, api.StopOptions{Timeout: &timeout}); err != nil {
		return err
	}
	return WaitStopped(ctx, docker.Client(), api.ProjectLabel+"="+projectName, stopTimeout)
}

// ComposeDown removes the containers of the app, its volumes, the images it built and its
// leftovers.
func ComposeDown(ctx context.Context, docker command.Cli, projectName string, line func(string)) error {
	backend, err := compose.NewComposeService(docker, compose.WithEventProcessor(newProgress(line)))
	if err != nil {
		return err
	}
	timeout := stopTimeout
	err = backend.Down(ctx, projectName, api.DownOptions{
		RemoveOrphans: true,
		Volumes:       true,
		Images:        "local",
		Timeout:       &timeout,
	})
	if err != nil {
		return err
	}
	pruneDanglingBuilds(ctx, docker, projectName)
	return WaitStopped(ctx, docker.Client(), api.ProjectLabel+"="+projectName, stopTimeout)
}

// classicBuilder makes the sdk build with the legacy builder, and writes the build output
// to out. The sdk needs buildx 0.17 for BuildKit, and the UNO Q has 0.13.
type classicBuilder struct {
	command.Cli
	out *streams.Out
}

func (classicBuilder) BuildKitEnabled() (bool, error) { return false, nil }

func (b classicBuilder) Out() *streams.Out { return b.out }

const builtImageLabel = "cc.arduino.app.built"

// pruneDanglingBuilds removes the untagged images the builds of the project left: the
// image a rebuild replaced, and the parents of the one a down removed.
func pruneDanglingBuilds(ctx context.Context, docker command.Cli, projectName string) {
	_, err := docker.Client().ImagePrune(ctx, dockerClient.ImagePruneOptions{
		Filters: make(dockerClient.Filters).
			Add("dangling", "true").
			Add("label", builtImageLabel+"=true").
			Add("label", api.ProjectLabel+"="+projectName),
	})
	if err != nil {
		slog.Warn("cannot remove the untagged images of the builds", slog.String("project", projectName), slog.String("error", err.Error()))
	}
}

// stampComposeLabels marks the services and their built images the way the sdk marks the
// projects it loads: compose finds the containers of a project by these labels.
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
		if service.Build != nil {
			service.Build.Labels = service.Build.Labels.
				Add(builtImageLabel, "true").
				Add(api.ProjectLabel, prj.Name).
				Add(api.ServiceLabel, name)
		}
		prj.Services[name] = service
	}
}

// composeProgress is where the sdk reports to: an api.EventProcessor, and the writer of
// the build output.
type composeProgress struct {
	line          func(string)
	mu            sync.Mutex
	said          map[string]string
	build         []byte
	registryError error
	networkError  error
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
		if event.Status == api.Error && strings.HasPrefix(event.ID, "Network ") {
			p.networkError = errors.New(event.Details)
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

// Write says each complete line of the build output; the rest waits for its newline.
func (p *composeProgress) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.build = append(p.build, b...)
	for {
		before, after, found := bytes.Cut(p.build, []byte("\n"))
		if !found {
			break
		}
		if line := strings.TrimSpace(string(before)); line != "" {
			p.line(line)
		}
		p.build = after
	}
	return len(b), nil
}

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
