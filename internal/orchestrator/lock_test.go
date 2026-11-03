// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestAppsLockIsReleasedAfterAFailure(t *testing.T) {
	cfg := setTestOrchestratorConfig(t)
	failure := errors.New("the operation failed")
	require.ErrorIs(t, withAppsLock(t.Context(), cfg, func() error { return failure }), failure)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	require.NoError(t, withAppsLock(ctx, cfg, func() error { return nil }))
}
