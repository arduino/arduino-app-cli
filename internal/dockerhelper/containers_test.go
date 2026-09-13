// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package dockerhelper

import (
	"context"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/events"
	dockerClient "github.com/moby/moby/client"
	"github.com/stretchr/testify/require"
)

// A label of this test alone, so a board running it keeps whatever else it has.
const testLabel = "cc.arduino.app.test"

func TestContainers(t *testing.T) {
	docker := getDockerCli(t).Client()
	running := startLabelled(t, docker, "running")

	found, err := Containers(t.Context(), docker, testLabel+"=running")
	require.NoError(t, err)
	require.Len(t, found, 1)
	require.Equal(t, running, found[0].ID)
	require.Equal(t, container.StateRunning, found[0].State)

	other, err := Containers(t.Context(), docker, testLabel+"=absent")
	require.NoError(t, err)
	require.Empty(t, other)
}

func TestStopContainers(t *testing.T) {
	docker := getDockerCli(t).Client()
	id := startLabelled(t, docker, "stop")

	stopped, err := StopContainers(t.Context(), docker, testLabel+"=stop")
	require.NoError(t, err)
	require.Equal(t, 1, stopped)

	state, err := docker.ContainerInspect(t.Context(), id, dockerClient.ContainerInspectOptions{})
	require.NoError(t, err)
	require.Equal(t, container.StateExited, state.Container.State.Status)
}

func TestPruneContainers(t *testing.T) {
	docker := getDockerCli(t).Client()
	startLabelled(t, docker, "prune")
	startLabelled(t, docker, "keep")

	// A running container is removed too, and what keep refuses is left alone.
	pruned, err := PruneContainers(t.Context(), docker, testLabel, func(labels map[string]string) bool {
		return labels[testLabel] == "prune"
	})
	require.NoError(t, err)
	require.Equal(t, 1, pruned)

	left, err := Containers(t.Context(), docker, testLabel)
	require.NoError(t, err)
	require.Len(t, left, 1)
	require.Equal(t, "keep", left[0].Labels[testLabel])
}

func TestPruneNetworks(t *testing.T) {
	docker := getDockerCli(t).Client()
	created, err := docker.NetworkCreate(t.Context(), "arduino-app-cli-test-network", dockerClient.NetworkCreateOptions{
		Labels: map[string]string{testLabel: "network"},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = docker.NetworkRemove(context.WithoutCancel(t.Context()), created.ID, dockerClient.NetworkRemoveOptions{})
	})

	kept, err := PruneNetworks(t.Context(), docker, testLabel, func(map[string]string) bool { return false })
	require.NoError(t, err)
	require.Zero(t, kept)

	pruned, err := PruneNetworks(t.Context(), docker, testLabel, nil)
	require.NoError(t, err)
	require.Equal(t, 1, pruned)
}

func TestContainerEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	docker := getDockerCli(t).Client()

	messages, errs := ContainerEvents(ctx, docker, testLabel+"=events", "start", "die")
	id := startLabelled(t, docker, "events")

	select {
	case event := <-messages:
		require.Equal(t, id, event.Actor.ID)
		require.Equal(t, events.Action("start"), event.Action)
	case err := <-errs:
		t.Fatal(err)
	case <-time.After(30 * time.Second):
		t.Fatal("no event for a container that started")
	}
}

// startLabelled runs a container that stays up until the test removes it.
func startLabelled(t *testing.T, docker dockerClient.APIClient, label string) string {
	t.Helper()
	require.NoError(t, ensureImage(t.Context(), docker, "busybox:latest"))

	created, err := docker.ContainerCreate(t.Context(), dockerClient.ContainerCreateOptions{
		Config: &container.Config{
			Image:  "busybox:latest",
			Cmd:    []string{"sleep", "600"},
			Labels: map[string]string{testLabel: label},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		// The context of a test is canceled before its cleanups run.
		_, _ = docker.ContainerRemove(context.WithoutCancel(t.Context()), created.ID, dockerClient.ContainerRemoveOptions{Force: true})
	})

	_, err = docker.ContainerStart(t.Context(), created.ID, dockerClient.ContainerStartOptions{})
	require.NoError(t, err)
	return created.ID
}
