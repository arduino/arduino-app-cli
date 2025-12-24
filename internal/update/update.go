// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package update

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
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

type ServiceUpdater interface {
	ListUpgradablePackages(ctx context.Context, matcher func(UpgradablePackage) bool) ([]UpgradablePackage, error)
	UpgradePackages(ctx context.Context, names []string, eventCB func(Event)) error
}

type Manager struct {
	lock                         sync.Mutex
	debUpdateService             ServiceUpdater
	arduinoPlatformUpdateService ServiceUpdater

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

func (m *Manager) ListUpgradablePackages(ctx context.Context, matcher func(UpgradablePackage) bool) ([]UpgradablePackage, error) {
	if !m.lock.TryLock() {
		return nil, ErrOperationAlreadyInProgress
	}
	defer m.lock.Unlock()

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
	if !m.lock.TryLock() {
		return ErrOperationAlreadyInProgress
	}
	ctx = context.WithoutCancel(ctx)
	var debPkgs []string
	var arduinoPlatform []string
	for _, v := range pkgs {
		switch v.Type {
		case Arduino:
			arduinoPlatform = append(arduinoPlatform, v.Name)
		case Debian:
			debPkgs = append(debPkgs, v.Name)
		default:
			return fmt.Errorf("unknown package type %s", v.Type)
		}
	}

	go func() {
		defer m.lock.Unlock()

		// We are launching on purpose the update sequentially. The reason is that
		// the deb pkgs restart the orchestrator, and if we run in parallel the
		// update of the cores we will end up with inconsistent state, or
		// we need to re run the upgrade because the orchestrator interrupted
		// in the middle the upgrade of the cores.
		if err := m.arduinoPlatformUpdateService.UpgradePackages(ctx, arduinoPlatform, m.broadcast); err != nil {
			m.broadcast(NewErrorEvent(fmt.Errorf("failed to upgrade Arduino packages: %w", err)))

			// continue with deb packages upgrade.
		}

		if err := m.debUpdateService.UpgradePackages(ctx, debPkgs, m.broadcast); err != nil {
			m.broadcast(NewErrorEvent(fmt.Errorf("failed to upgrade APT packages: %w", err)))
			return
		}

		m.broadcast(NewDataEvent(DoneEvent, "Update completed"))
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
