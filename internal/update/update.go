// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package update

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

var MatchArduinoPackage = func(p UpgradablePackage) bool {
	return strings.HasPrefix(p.Name, "arduino-") ||
		(p.Name == "adbd" && strings.Contains(p.ToVersion, "arduino")) // NOTE: changing this check could remove the adbd package, breaking the device access.
}

var MatchAllPackages = func(p UpgradablePackage) bool {
	return true
}

type UpgradablePackage struct {
	Type         PackageType `json:"type"` // e.g., "arduino", "deb"
	Name         string      `json:"name"` // Package name without repository information
	Architecture string      `json:"-"`
	FromVersion  string      `json:"from_version"`
	ToVersion    string      `json:"to_version"`
}

type PackageInfo struct {
	Name      string
	ToVersion string
}

type ServiceUpdater interface {
	ListUpgradablePackages(ctx context.Context, matcher func(UpgradablePackage) bool) ([]UpgradablePackage, error)
	UpgradePackages(ctx context.Context, packages []PackageInfo, eventCB EventCallback) error
}

// The deb package of this very program: upgrading it needs a process restart.
const selfPackageName = "arduino-app-cli"

type Manager struct {
	lock                         sync.Mutex
	isUpgrading                  atomic.Bool
	debUpdateService             ServiceUpdater
	arduinoPlatformUpdateService ServiceUpdater
	selfRestart                  bool

	mu   sync.RWMutex
	subs map[chan Event]struct{}
}

func NewManager(debUpdateService ServiceUpdater, arduinoPlatformUpdateService ServiceUpdater) *Manager {
	return &Manager{
		debUpdateService:             debUpdateService,
		arduinoPlatformUpdateService: arduinoPlatformUpdateService,
		subs:                         make(map[chan Event]struct{}),
	}
}

// The daemon is restarted after a self-upgrade either way: only it may take the
// shortcut of killing itself, since systemd respawns it. The CLI must not.
func (m *Manager) WithSelfRestart() *Manager {
	m.selfRestart = true
	return m
}

func (m *Manager) ListUpgradablePackages(ctx context.Context, matcher func(UpgradablePackage) bool) ([]UpgradablePackage, error) {
	// Atomically check if an upgrade operation is already in progress. See https://github.com/arduino/arduino-app-cli/issues/381.
	if m.isUpgrading.Load() {
		return nil, ErrOperationAlreadyInProgress
	}

	// Make sure to be connected to the internet, before checking for updates.
	// This is needed because the checks below work also when offline (using cached data).
	if !isConnected() {
		return nil, ErrNoInternetConnection
	}

	// Get the list of upgradable packages from two sources (deb and platform) in parallel.
	g, ctx := errgroup.WithContext(ctx)
	var (
		debPkgs     []UpgradablePackage
		arduinoPkgs []UpgradablePackage
	)

	g.Go(func() error {
		pkgs, err := m.debUpdateService.ListUpgradablePackages(ctx, matcher)
		if err != nil {
			return err
		}
		debPkgs = pkgs
		return nil
	})

	g.Go(func() error {
		pkgs, err := m.arduinoPlatformUpdateService.ListUpgradablePackages(ctx, matcher)
		if err != nil {
			return err
		}
		arduinoPkgs = pkgs
		return nil
	})

	// Wait for all the checks to complete (or any to fail).
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return append(arduinoPkgs, debPkgs...), nil
}

func (m *Manager) UpgradePackages(ctx context.Context, pkgs []UpgradablePackage) error {
	ctx = context.WithoutCancel(ctx)
	var debPkgs []PackageInfo
	var arduinoPlatform []PackageInfo
	for _, v := range pkgs {
		switch v.Type {
		case Arduino:
			arduinoPlatform = append(arduinoPlatform, PackageInfo{Name: v.Name, ToVersion: v.ToVersion})
		case Debian:
			debPkgs = append(debPkgs, PackageInfo{Name: v.Name, ToVersion: v.ToVersion})
		default:
			return fmt.Errorf("unknown package type %s", v.Type)
		}
	}

	if !m.lock.TryLock() {
		return ErrOperationAlreadyInProgress
	}
	m.isUpgrading.Store(true)
	go func() {
		defer m.lock.Unlock()
		defer m.isUpgrading.Store(false)

		const arduinoWeight float32 = 20.0
		const aptWeight float32 = 80.0

		// We are launching on purpose the update sequentially. The reason is that
		// the deb pkgs restart the orchestrator, and if we run in parallel the
		// update of the cores we will end up with inconsistent state, or
		// we need to re run the upgrade because the orchestrator interrupted
		// in the middle the upgrade of the cores.
		if err := m.arduinoPlatformUpdateService.UpgradePackages(ctx, arduinoPlatform, func(e Event) {
			if e.Type == ProgressEvent {
				progress := e.GetProgress()
				globalProgress := (progress.Progress / 100.0) * arduinoWeight
				m.broadcast(NewProgressEvent(progress.Step, globalProgress))
			} else {
				m.broadcast(e)
			}
		}); err != nil {
			m.broadcast(NewErrorEvent(fmt.Errorf("failed to upgrade Arduino packages: %w", err)))

			// continue with deb packages upgrade.
		}

		if err := m.debUpdateService.UpgradePackages(ctx, debPkgs, func(e Event) {
			if e.Type == ProgressEvent {
				progress := e.GetProgress()
				globalProgress := arduinoWeight + (progress.Progress/100.0)*aptWeight
				m.broadcast(NewProgressEvent(progress.Step, globalProgress))
			} else {
				m.broadcast(e)
			}
		}); err != nil {
			m.broadcast(NewErrorEvent(fmt.Errorf("failed to upgrade APT packages: %w", err)))

			// continue: errors are reported to the subscribers but do not end the
			// operation, DoneEvent is always broadcast as the only terminal event.
		}

		m.broadcast(NewProgressEvent("upgrade", 100.0))

		m.broadcast(NewDataEvent(DoneEvent, "Update completed"))

		isSelfUpgrade := slices.ContainsFunc(debPkgs, func(p PackageInfo) bool { return p.Name == selfPackageName })
		if m.selfRestart && isSelfUpgrade {
			m.broadcast(NewDataEvent(RestartEvent, fmt.Sprintf("Upgrade completed. Restarting (pid %d) ...", os.Getpid())))
			// needrestart skips its caller's cgroup, so we signal ourselves to let
			// systemd respawn us on the new binary, after the last broadcast.
			if p, err := os.FindProcess(os.Getpid()); err == nil {
				if err := p.Signal(syscall.SIGTERM); err != nil {
					slog.Error("failed to send SIGTERM to self after upgrade", slog.String("error", err.Error()))
				}
			}
		}
	}()
	return nil
}

// Subscribe creates a new channel for receiving APT events.
func (b *Manager) Subscribe() chan Event {
	eventCh := make(chan Event, 100)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[eventCh] = struct{}{}
	return eventCh
}

// Unsubscribe removes the channel from the list of subscribers and closes it.
func (b *Manager) Unsubscribe(eventCh chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subs, eventCh)
	close(eventCh)
}

func (b *Manager) broadcast(event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if event.Type == ErrorEvent {
		slog.Error("An error occurred", slog.Any("event", event))
	}
	for ch := range b.subs {
		select {
		case ch <- event:
		default:
			slog.Warn("Discarding event (channel full)",
				slog.String("type", event.Type.String()),
				slog.Any("event", event),
			)
		}
	}
}

func isConnected() bool {
	client := http.Client{
		Timeout: 3 * time.Second,
	}

	// Just check that the connection can be estabilished.
	// The HEAD method will not get the results and we are ignoring the HTTP status code.
	resp, err := client.Head("https://downloads.arduino.cc/")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return true
}
