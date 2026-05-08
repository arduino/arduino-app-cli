// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package update

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

type mockServiceUpdater struct {
	checkFn func(ctx context.Context, matcher func(UpgradablePackage) bool) ([]UpgradablePackage, error)
	applyFn func(ctx context.Context, packages []PackageInfo, eventCB EventCallback) error
}

func (m *mockServiceUpdater) ListUpgradablePackages(ctx context.Context, matcher func(UpgradablePackage) bool) ([]UpgradablePackage, error) {
	if m.checkFn != nil {
		return m.checkFn(ctx, matcher)
	}
	return nil, nil
}

func (m *mockServiceUpdater) UpgradePackages(ctx context.Context, pkgs []PackageInfo, cb EventCallback) error {
	if m.applyFn != nil {
		return m.applyFn(ctx, pkgs, cb)
	}
	return nil
}

func newTestManager(deb, arduino ServiceUpdater) *Manager {
	return &Manager{
		debUpdateService:             deb,
		arduinoPlatformUpdateService: arduino,
		subs:                         make(map[chan Event]struct{}),
	}
}

func TestManagerListUpgradablePackages(t *testing.T) {
	tests := []struct {
		name           string
		svc1Packages   []UpgradablePackage
		svc1Err        error
		svc2Packages   []UpgradablePackage
		svc2Err        error
		expectErr      bool
		expectPackages int
	}{
		{
			name:           "both services return packages",
			svc1Packages:   []UpgradablePackage{{Name: "pkg1", FromVersion: "1.0", ToVersion: "2.0"}},
			svc2Packages:   []UpgradablePackage{{Name: "pkg2", FromVersion: "0.1", ToVersion: "0.2"}},
			expectPackages: 2,
		},
		{
			name:           "only deb service returns packages",
			svc1Packages:   []UpgradablePackage{{Name: "pkg1", FromVersion: "1.0", ToVersion: "2.0"}},
			svc2Packages:   nil,
			expectPackages: 1,
		},
		{
			name:           "only arduino service returns packages",
			svc1Packages:   nil,
			svc2Packages:   []UpgradablePackage{{Name: "pkg2", FromVersion: "0.1", ToVersion: "0.2"}},
			expectPackages: 1,
		},
		{
			name:           "both services return no packages",
			svc1Packages:   nil,
			svc2Packages:   nil,
			expectPackages: 0,
		},
		{
			name:      "deb service fails",
			svc1Err:   errors.New("network error"),
			expectErr: true,
		},
		{
			name:      "arduino service fails",
			svc2Err:   errors.New("network error"),
			expectErr: true,
		},
		{
			name:      "both services fail",
			svc1Err:   errors.New("error 1"),
			svc2Err:   errors.New("error 2"),
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc1 := &mockServiceUpdater{
				checkFn: func(ctx context.Context, matcher func(UpgradablePackage) bool) ([]UpgradablePackage, error) {
					return tc.svc1Packages, tc.svc1Err
				},
			}
			svc2 := &mockServiceUpdater{
				checkFn: func(ctx context.Context, matcher func(UpgradablePackage) bool) ([]UpgradablePackage, error) {
					return tc.svc2Packages, tc.svc2Err
				},
			}
			m := newTestManager(svc1, svc2)

			results, err := m.ListUpgradablePackages(context.Background(), nil)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, results, tc.expectPackages)
		})
	}
}

func TestManagerListUpgradablePackagesMultipleConcurrentChecks(t *testing.T) {
	var svc1Calls, svc2Calls atomic.Int32
	svc1 := &mockServiceUpdater{
		checkFn: func(ctx context.Context, matcher func(UpgradablePackage) bool) ([]UpgradablePackage, error) {
			svc1Calls.Add(1)
			time.Sleep(50 * time.Millisecond)
			return []UpgradablePackage{{Name: "pkg1"}}, nil
		},
	}
	svc2 := &mockServiceUpdater{
		checkFn: func(ctx context.Context, matcher func(UpgradablePackage) bool) ([]UpgradablePackage, error) {
			svc2Calls.Add(1)
			time.Sleep(50 * time.Millisecond)
			return []UpgradablePackage{{Name: "pkg2"}}, nil
		},
	}
	m := newTestManager(svc1, svc2)

	const n = 5
	var wg sync.WaitGroup
	wg.Add(n)
	errs := make([]error, n)
	for i := range n {
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = m.ListUpgradablePackages(context.Background(), nil)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		assert.NoErrorf(t, err, "goroutine %d returned unexpected error", i)
	}
	assert.Equal(t, int32(n), svc1Calls.Load(), "svc1 call count mismatch")
	assert.Equal(t, int32(n), svc2Calls.Load(), "svc2 call count mismatch")

	// Now start an upgrade and verify check returns ErrOperationAlreadyInProgress
	started := make(chan struct{})
	svc1.applyFn = func(ctx context.Context, pkgs []PackageInfo, cb EventCallback) error {
		return nil
	}
	svc2.applyFn = func(ctx context.Context, pkgs []PackageInfo, cb EventCallback) error {
		close(started)
		time.Sleep(200 * time.Millisecond)
		return nil
	}

	err := m.UpgradePackages(context.Background(), []UpgradablePackage{{Type: Debian, Name: "pkg1", ToVersion: "2.0"}})
	require.NoError(t, err, "unexpected error starting upgrade")
	<-started

	_, err = m.ListUpgradablePackages(context.Background(), nil)
	require.ErrorIs(t, err, ErrOperationAlreadyInProgress, "expected ErrOperationAlreadyInProgress during upgrade")

	// Wait for upgrade goroutine to finish
	time.Sleep(300 * time.Millisecond)

	// After upgrade completes, check should work again
	_, err = m.ListUpgradablePackages(context.Background(), nil)
	require.NoError(t, err, "unexpected error after upgrade completed")
}

func TestManagerUpgradePackages(t *testing.T) {
	tests := []struct {
		name       string
		packages   []UpgradablePackage
		svc1Err    error
		svc2Err    error
		expectEvts []EventType // expected last event type from broadcast
	}{
		{
			name:       "both services upgrade successfully",
			packages:   []UpgradablePackage{{Type: Arduino, Name: "arduino:zephyr", ToVersion: "2.0"}, {Type: Debian, Name: "pkg1", ToVersion: "1.1"}},
			expectEvts: []EventType{DoneEvent},
		},
		{
			name:       "arduino service fails, deb succeeds",
			packages:   []UpgradablePackage{{Type: Arduino, Name: "arduino:zephyr", ToVersion: "2.0"}, {Type: Debian, Name: "pkg1", ToVersion: "1.1"}},
			svc1Err:    errors.New("arduino upgrade failed"),
			expectEvts: []EventType{ErrorEvent, DoneEvent}, // should continue to deb upgrade and complete
		},
		{
			name:     "deb service fails",
			packages: []UpgradablePackage{{Type: Debian, Name: "pkg1", ToVersion: "1.1"}},
			svc2Err:  errors.New("deb upgrade failed"),
			// FIXME: we should alwas return Done?
			expectEvts: []EventType{ErrorEvent},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc1 := &mockServiceUpdater{
				applyFn: func(ctx context.Context, pkgs []PackageInfo, cb EventCallback) error {
					return tc.svc1Err
				},
			}
			svc2 := &mockServiceUpdater{
				applyFn: func(ctx context.Context, pkgs []PackageInfo, cb EventCallback) error {
					return tc.svc2Err
				},
			}
			m := newTestManager(svc2, svc1) // deb first, arduino second

			ch := m.Subscribe()
			defer m.Unsubscribe(ch)

			err := m.UpgradePackages(context.Background(), tc.packages)
			require.NoError(t, err, "unexpected error starting upgrade")

			// Collect the last event
			timeout := time.After(1 * time.Second)
			var events []EventType
		loop:
			for {
				select {
				case ev := <-ch:
					events = append(events, ev.Type)
					if ev.Type == DoneEvent {
						break loop
					}
				case <-timeout:
					break loop
				}
			}

			assert.Equal(t, tc.expectEvts, events, "unexpected event sequence")
		})
	}
}

func TestManagerUpgradePackagesConcurrentUpgradeReturnsError(t *testing.T) {
	started := make(chan struct{})
	svc1 := &mockServiceUpdater{
		applyFn: func(ctx context.Context, pkgs []PackageInfo, cb EventCallback) error {
			close(started)
			time.Sleep(200 * time.Millisecond)
			return nil
		},
	}
	svc2 := &mockServiceUpdater{
		applyFn: func(ctx context.Context, pkgs []PackageInfo, cb EventCallback) error {
			return nil
		},
	}
	m := newTestManager(svc1, svc2)

	pkgs := []UpgradablePackage{{Type: Debian, Name: "pkg1", ToVersion: "2.0"}}

	err := m.UpgradePackages(context.Background(), pkgs)
	assert.NoError(t, err, "first upgrade should start without error")
	<-started

	// Second upgrade should fail
	err = m.UpgradePackages(context.Background(), pkgs)
	assert.ErrorIs(t, err, ErrOperationAlreadyInProgress, "expected ErrOperationAlreadyInProgress for concurrent upgrade")
}

func TestManagerSubscribeReceivesUpgradeEvents(t *testing.T) {
	svc1 := &mockServiceUpdater{
		applyFn: func(ctx context.Context, pkgs []PackageInfo, cb EventCallback) error {
			cb(NewDataEvent(StartEvent, "starting arduino"))
			cb(NewDataEvent(UpgradeLineEvent, "upgrading arduino"))
			return nil
		},
	}
	svc2 := &mockServiceUpdater{
		applyFn: func(ctx context.Context, pkgs []PackageInfo, cb EventCallback) error {
			cb(NewDataEvent(StartEvent, "starting deb"))
			cb(NewDataEvent(UpgradeLineEvent, "upgrading deb"))
			return nil
		},
	}
	m := newTestManager(svc2, svc1) // deb, arduino

	ch := m.Subscribe()
	defer m.Unsubscribe(ch)

	pkgs := []UpgradablePackage{
		{Type: Arduino, Name: "arduino:zephyr", ToVersion: "2.0"},
		{Type: Debian, Name: "pkg1", ToVersion: "1.1"},
	}
	err := m.UpgradePackages(context.Background(), pkgs)
	assert.NoError(t, err, "unexpected error starting upgrade")

	var events []Event
	timeout := time.After(2 * time.Second)
	// 2 from arduino + 2 from deb + 1 DoneEvent = 5
	for range 5 {
		select {
		case ev := <-ch:
			events = append(events, ev)
		case <-timeout:
			t.Fatalf("timeout waiting for events, got %d: %v", len(events), events)
		}
	}

	assert.Len(t, events, 5, "expected 5 events")
	assert.Equal(t, StartEvent, events[0].Type, "expected first event to be StartEvent")
	assert.Equal(t, DoneEvent, events[len(events)-1].Type, "expected first event to be StartEvent")
}
