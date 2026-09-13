// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package dockerhelper

import (
	"context"
	"fmt"

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

// PruneNetworks removes the networks carrying the label, keep having the last word.
func PruneNetworks(ctx context.Context, docker dockerClient.APIClient, label string, keep func(labels map[string]string) bool) (int, error) {
	networks, err := docker.NetworkList(ctx, dockerClient.NetworkListOptions{
		Filters: make(dockerClient.Filters).Add("label", label),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list networks: %w", err)
	}

	var pruned int
	for _, info := range networks.Items {
		if keep != nil && !keep(info.Labels) {
			continue
		}
		if _, err := docker.NetworkRemove(ctx, info.ID, dockerClient.NetworkRemoveOptions{}); err != nil {
			return 0, fmt.Errorf("failed to remove network %s: %w", info.ID, err)
		}
		pruned++
	}
	return pruned, nil
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
