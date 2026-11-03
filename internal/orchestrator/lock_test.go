// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

func TestAppsLockSerializes(t *testing.T) {
	cfg := setTestOrchestratorConfig(t)

	var inside, overlaps atomic.Int32
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			assert.NoError(t, withAppsLock(t.Context(), cfg, func() error {
				if inside.Add(1) > 1 {
					overlaps.Add(1)
				}
				time.Sleep(100 * time.Millisecond)
				inside.Add(-1)
				return nil
			}))
		})
	}
	wg.Wait()
	assert.Zero(t, overlaps.Load())
}

func TestAppsLockWaitIsCancellable(t *testing.T) {
	cfg := setTestOrchestratorConfig(t)
	held, release := make(chan struct{}), make(chan struct{})
	go func() {
		_ = withAppsLock(t.Context(), cfg, func() error {
			close(held)
			<-release
			return nil
		})
	}()
	<-held
	defer close(release)

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	called := false
	err := withAppsLock(ctx, cfg, func() error {
		called = true
		return nil
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.False(t, called)
}

func TestAppsLockIsNotInheritedByChildProcesses(t *testing.T) {
	cfg := setTestOrchestratorConfig(t)
	// A compiler or an upload tool started during an operation must not keep the lock
	// once the operation is over.
	child := exec.Command("sleep", "30")
	require.NoError(t, withAppsLock(t.Context(), cfg, child.Start))
	t.Cleanup(func() { _ = child.Process.Kill(); _, _ = child.Process.Wait() })

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	require.NoError(t, withAppsLock(ctx, cfg, func() error { return nil }), "the lock is free while the child still runs")
}

func testApp(t *testing.T) app.ArduinoApp {
	return app.ArduinoApp{FullPath: paths.New(t.TempDir())}
}

func TestAppOperationsShareTheRunOfEqualRequests(t *testing.T) {
	cfg := setTestOrchestratorConfig(t)
	one := testApp(t)
	failure := errors.New("the start failed")

	var runs atomic.Int32
	entered, release := make(chan struct{}), make(chan struct{})
	results := make(chan error, 2)
	go func() {
		results <- runExclusive(t.Context(), cfg, one, "start", func() error {
			runs.Add(1)
			close(entered)
			<-release
			return failure
		})
	}()
	<-entered
	go func() {
		results <- runExclusive(t.Context(), cfg, one, "start", func() error {
			runs.Add(1)
			return errors.New("an equal request must not run")
		})
	}()
	time.Sleep(50 * time.Millisecond)
	close(release)
	require.ErrorIs(t, <-results, failure)
	require.ErrorIs(t, <-results, failure, "the second request got the result of the first")
	assert.Equal(t, int32(1), runs.Load())
}

func TestAppOperationsRunAgainWhenTheSharedRunIsCanceled(t *testing.T) {
	cfg := setTestOrchestratorConfig(t)
	one := testApp(t)

	leaderCtx, cancelLeader := context.WithCancel(t.Context())
	defer cancelLeader()
	var runs atomic.Int32
	entered := make(chan struct{})
	leader := make(chan error, 1)
	go func() {
		leader <- runExclusive(leaderCtx, cfg, one, "start", func() error {
			runs.Add(1)
			close(entered)
			<-leaderCtx.Done()
			return leaderCtx.Err()
		})
	}()
	<-entered
	follower := make(chan error, 1)
	go func() {
		follower <- runExclusive(t.Context(), cfg, one, "start", func() error {
			runs.Add(1)
			return nil
		})
	}()
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, int32(1), runs.Load(), "the follower waits")

	cancelLeader()
	require.ErrorIs(t, <-leader, context.Canceled)
	require.NoError(t, <-follower, "the follower runs the operation itself")
	assert.Equal(t, int32(2), runs.Load())
}

func TestAppOperationsLeaveNothingBehindAfterAPanic(t *testing.T) {
	cfg := setTestOrchestratorConfig(t)
	one := testApp(t)

	panicked := func() (recovered any) {
		defer func() { recovered = recover() }()
		_ = runExclusive(t.Context(), cfg, one, "start", func() error { panic("the start blew up") })
		return nil
	}()
	assert.Contains(t, fmt.Sprint(panicked), "the start blew up", "the panic reaches the request")
	require.NoError(t, runExclusive(t.Context(), cfg, one, "start", func() error { return nil }), "the next request runs")
}
