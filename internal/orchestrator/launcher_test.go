// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/arduino/go-paths-helper"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/moby/moby/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/launcher"
)

func TestRuntimeKeyFitsASocketPath(t *testing.T) {
	assert.Equal(t, "my-app", runtimeKey("my-app"))
	long := strings.Repeat("a-very-long-app-name-", 5)
	key := runtimeKey(long)
	assert.LessOrEqual(t, len(key), maxRuntimeKeyLength)
	assert.True(t, strings.HasPrefix(key, "a-very-long-app-name-"))
	assert.NotEqual(t, key, runtimeKey(long+"x"), "two long names are told apart")
}

func TestLauncherServedReadsTheMainServiceLabel(t *testing.T) {
	served := &types.Project{Services: types.Services{mainServiceName: {Labels: types.Labels{DockerAppLauncherLabel: "true"}}}}
	assert.True(t, launcherServed(served))
	classic := &types.Project{Services: types.Services{mainServiceName: {Labels: types.Labels{DockerAppMainLabel: "true"}}}}
	assert.False(t, launcherServed(classic), "a release frozen before the launcher runs its app the classic way")
	assert.False(t, launcherServed(&types.Project{}))
}

func TestTheLauncherDecidesTheStatusOfTheAppItDrives(t *testing.T) {
	appPath := paths.New("/apps/one").String()
	main := container.Summary{ID: "main", State: container.StateRunning, Labels: map[string]string{
		DockerAppPathLabel: appPath, DockerAppMainLabel: "true", DockerAppLauncherLabel: "true",
	}}
	sidecar := container.Summary{ID: "side", State: container.StateRunning, Labels: map[string]string{DockerAppPathLabel: appPath}}

	for _, tc := range []struct {
		name     string
		launcher Status
		expected Status
	}{
		{"an app the launcher runs", StatusRunning, StatusRunning},
		{"an app that crashed in a container that stays up", StatusFailed, StatusFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			statuses := parseAppStatus([]container.Summary{main, sidecar}, map[string]Status{"main": tc.launcher})
			require.Len(t, statuses, 1)
			assert.Equal(t, tc.expected, statuses[0].Status)
		})
	}

	t.Run("without an answer the container state stands", func(t *testing.T) {
		statuses := parseAppStatus([]container.Summary{main, sidecar}, nil)
		require.Len(t, statuses, 1)
		assert.Equal(t, StatusRunning, statuses[0].Status)
	})
}

func TestAnAppThatEndedInAContainerStillUpHasFailed(t *testing.T) {
	code, sigkill := 0, 9
	for _, tc := range []struct {
		name     string
		reported launcher.AppState
		expected Status
	}{
		{"an app that runs", launcher.AppState{Child: &launcher.Child{}}, StatusRunning},
		{"an app that exited", launcher.AppState{Last: &launcher.Ended{ExitCode: &code}}, StatusFailed},
		{"an app killed by a signal", launcher.AppState{Last: &launcher.Ended{Signal: &sigkill}}, StatusFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, known := statusOfLaunchedApp(tc.reported)
			assert.True(t, known)
			assert.Equal(t, tc.expected, status)
		})
	}

	t.Run("a launcher that ran no app yet leaves the container state to stand", func(t *testing.T) {
		_, known := statusOfLaunchedApp(launcher.AppState{})
		assert.False(t, known)
	})
}

// fakeLaunchers stands in for the launchers of the containers: each call of the events of a
// container plays the next of its scripted connections, then holds until it is dropped.
type fakeLaunchers struct {
	mu          sync.Mutex
	connections map[string][]func(onState func(launcher.AppState)) error
	asked       map[string]int
	dropped     chan string
}

func newFakeLaunchers() *fakeLaunchers {
	return &fakeLaunchers{connections: map[string][]func(func(launcher.AppState)) error{}, asked: map[string]int{}, dropped: make(chan string, 8)}
}

func (f *fakeLaunchers) events(ctx context.Context, c container.Summary, onState func(launcher.AppState)) error {
	f.mu.Lock()
	n := f.asked[c.ID]
	f.asked[c.ID]++
	connections := f.connections[c.ID]
	f.mu.Unlock()
	if n < len(connections) {
		if err := connections[n](onState); err != nil {
			return err
		}
	}
	<-ctx.Done()
	f.dropped <- c.ID
	return ctx.Err()
}

func (f *fakeLaunchers) timesAsked(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.asked[id]
}

func (f *fakeLaunchers) watch() *launcherWatch {
	return &launcherWatch{events: f.events, retry: time.Millisecond, updates: make(chan launcherUpdate), followed: map[string]*followedLauncher{}}
}

func launcherContainer(id string) container.Summary {
	return container.Summary{ID: id, State: container.StateRunning, Labels: map[string]string{
		DockerAppPathLabel: "/apps/" + id, DockerAppMainLabel: "true", DockerAppLauncherLabel: "true",
	}}
}

func nextUpdate(t *testing.T, watch *launcherWatch) launcherUpdate {
	t.Helper()
	select {
	case update := <-watch.updates:
		return update
	case <-time.After(5 * time.Second):
		require.FailNow(t, "the launcher told nothing")
		return launcherUpdate{}
	}
}

func TestTheLauncherOfARunningContainerTellsWhenItsAppStartsOrEnds(t *testing.T) {
	code := 1
	fake := newFakeLaunchers()
	fake.connections["one"] = []func(func(launcher.AppState)) error{func(onState func(launcher.AppState)) error {
		onState(launcher.AppState{})
		onState(launcher.AppState{Child: &launcher.Child{}})
		onState(launcher.AppState{Last: &launcher.Ended{ExitCode: &code}})
		return nil
	}}
	watch := fake.watch()
	classic := container.Summary{ID: "classic", State: container.StateRunning, Labels: map[string]string{DockerAppMainLabel: "true"}}
	watch.sync(t.Context(), []container.Summary{launcherContainer("one"), classic})
	assert.Empty(t, watch.statuses(), "until the launcher tells, the container state stands")

	require.True(t, watch.apply(nextUpdate(t, watch)))
	assert.Empty(t, watch.statuses(), "a launcher that ran no app yet has nothing to tell")
	require.True(t, watch.apply(nextUpdate(t, watch)))
	assert.Equal(t, map[string]Status{"one": StatusRunning}, watch.statuses())
	require.True(t, watch.apply(nextUpdate(t, watch)))
	assert.Equal(t, map[string]Status{"one": StatusFailed}, watch.statuses())
	assert.Zero(t, fake.timesAsked("classic"), "a container without a launcher is not followed")
}

func TestAContainerThatStoppedIsNoLongerFollowed(t *testing.T) {
	fake := newFakeLaunchers()
	fake.connections["one"] = []func(func(launcher.AppState)) error{func(onState func(launcher.AppState)) error {
		onState(launcher.AppState{Child: &launcher.Child{}})
		return nil
	}}
	watch := fake.watch()
	watch.sync(t.Context(), []container.Summary{launcherContainer("one")})
	update := nextUpdate(t, watch)
	require.True(t, watch.apply(update))

	stopped := launcherContainer("one")
	stopped.State = container.StateExited
	watch.sync(t.Context(), []container.Summary{stopped})
	assert.Equal(t, "one", <-fake.dropped)
	assert.Empty(t, watch.statuses(), "the state of the stopped container stands")
	assert.False(t, watch.apply(update), "what its launcher told before is ignored")

	// The container runs again: its new launcher is followed anew
	watch.sync(t.Context(), []container.Summary{launcherContainer("one")})
	assert.Eventually(t, func() bool { return fake.timesAsked("one") == 2 }, 5*time.Second, time.Millisecond)
	assert.False(t, watch.apply(update), "nor is it taken for a word of the new one")
}

func TestALauncherIsAskedAgainUntilItGivesItsEvents(t *testing.T) {
	notUp := func(func(launcher.AppState)) error { return errors.New("cannot reach the app launcher") }
	fake := newFakeLaunchers()
	fake.connections["fresh"] = []func(func(launcher.AppState)) error{notUp, notUp, func(onState func(launcher.AppState)) error {
		onState(launcher.AppState{Child: &launcher.Child{}})
		return errors.New("connection closed")
	}}
	fake.connections["old"] = []func(func(launcher.AppState)) error{func(func(launcher.AppState)) error {
		return &launcher.Error{Code: launcher.CodeBadRequest, Message: "unknown cmd: 'events'"}
	}}
	watch := fake.watch()
	watch.sync(t.Context(), []container.Summary{launcherContainer("fresh"), launcherContainer("old")})

	require.True(t, watch.apply(nextUpdate(t, watch)))
	assert.Equal(t, map[string]Status{"fresh": StatusRunning}, watch.statuses())
	// The launcher went: it is asked again, and what it told stands until its container tells otherwise
	assert.Eventually(t, func() bool { return fake.timesAsked("fresh") == 4 }, 5*time.Second, time.Millisecond)
	assert.Equal(t, map[string]Status{"fresh": StatusRunning}, watch.statuses())
	assert.Equal(t, 1, fake.timesAsked("old"), "a launcher without events is not asked again")
}

func TestTheLauncherPartOfAStartOutlivesItsCaller(t *testing.T) {
	ctx, leave := context.WithCancel(t.Context())
	leave()

	stopped := false
	err := launchOrStop(ctx,
		func(ctx context.Context) error { return ctx.Err() },
		func(context.Context) error { stopped = true; return nil })

	require.NoError(t, err)
	require.False(t, stopped, "an app its launcher started is left running")
}

func TestAnAppItsLauncherDidNotStartIsStopped(t *testing.T) {
	ctx, leave := context.WithCancel(t.Context())
	leave()
	refused := errors.New("refused")

	var stopErr error
	stopped := false
	err := launchOrStop(ctx,
		func(context.Context) error { return refused },
		func(ctx context.Context) error { stopped, stopErr = true, ctx.Err(); return nil })

	require.ErrorIs(t, err, refused)
	require.True(t, stopped)
	require.NoError(t, stopErr, "the stop does not depend on the caller either")
}
