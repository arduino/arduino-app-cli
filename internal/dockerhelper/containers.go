// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package dockerhelper

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/events"
	dockerClient "github.com/moby/moby/client"
)

// PruneContainers removes the containers carrying the label, running ones included, and
// reports how many. A label cannot express everything, so keep has the last word.
func PruneContainers(ctx context.Context, docker dockerClient.APIClient, label string, keep func(labels map[string]string) bool) (int, error) {
	containers, err := docker.ContainerList(ctx, dockerClient.ContainerListOptions{
		All:     true,
		Filters: make(dockerClient.Filters).Add("label", label),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list containers: %w", err)
	}

	var pruned int
	for _, info := range containers.Items {
		if keep != nil && !keep(info.Labels) {
			continue
		}
		if _, err := docker.ContainerRemove(ctx, info.ID, dockerClient.ContainerRemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		}); err != nil {
			return 0, fmt.Errorf("failed to remove container %s: %w", info.ID, err)
		}
		pruned++
	}
	return pruned, nil
}

// PruneNetworks removes every network remove accepts, by its labels, and reports how
// many went.
func PruneNetworks(ctx context.Context, docker dockerClient.APIClient, remove func(labels map[string]string) bool) (int, error) {
	networks, err := docker.NetworkList(ctx, dockerClient.NetworkListOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to list networks: %w", err)
	}

	var pruned int
	var failed []error
	for _, info := range networks.Items {
		if !remove(info.Labels) {
			continue
		}
		// A network an app still runs on is refused: take the others anyway.
		if _, err := docker.NetworkRemove(ctx, info.ID, dockerClient.NetworkRemoveOptions{}); err != nil {
			failed = append(failed, fmt.Errorf("failed to remove network %s: %w", info.Name, err))
			continue
		}
		pruned++
	}
	return pruned, errors.Join(failed...)
}

// Containers lists the containers carrying the label, stopped ones included.
func Containers(ctx context.Context, docker dockerClient.APIClient, label string) ([]container.Summary, error) {
	containers, err := docker.ContainerList(ctx, dockerClient.ContainerListOptions{
		All:     true,
		Filters: make(dockerClient.Filters).Add("label", label),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	return containers.Items, nil
}

// WaitStopped waits until the engine reports no running container carrying the label. A stop
// returns once the container exited, the list the engine serves catches up a moment later, and
// a start right after a stop would still find the app running.
func WaitStopped(ctx context.Context, docker dockerClient.APIClient, label string, timeout time.Duration) error {
	started := time.Now()
	for {
		containers, err := Containers(ctx, docker, label)
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(containers, func(c container.Summary) bool { return c.State == container.StateRunning }) {
			if waited := time.Since(started); waited > time.Millisecond {
				slog.Debug("waited for the engine to report the containers stopped", slog.String("label", label), slog.Duration("wait", waited))
			}
			return nil
		}
		if time.Since(started) > timeout {
			return fmt.Errorf("the containers %s are still reported running %s after their stop", label, timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// StopContainers stops the containers carrying the label.
func StopContainers(ctx context.Context, docker dockerClient.APIClient, label string) (int, error) {
	containers, err := Containers(ctx, docker, label)
	if err != nil {
		return 0, err
	}

	var stopped int
	for _, info := range containers {
		if _, err := docker.ContainerStop(ctx, info.ID, dockerClient.ContainerStopOptions{}); err != nil {
			return stopped, fmt.Errorf("failed to stop container %s: %w", info.ID, err)
		}
		stopped++
	}
	return stopped, nil
}

// ContainerEvents streams the actions of the containers carrying the label, as
// `docker events` does: create, start, die, and whatever else the caller names.
func ContainerEvents(ctx context.Context, docker dockerClient.APIClient, label string, actions ...string) (<-chan events.Message, <-chan error) {
	stream := docker.Events(ctx, dockerClient.EventsListOptions{
		Filters: make(dockerClient.Filters).
			Add("label", label).
			Add("type", string(events.ContainerEventType)).
			Add("event", actions...),
	})
	return stream.Messages, stream.Err
}
