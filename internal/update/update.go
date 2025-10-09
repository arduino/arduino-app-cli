package update

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

var ErrOperationAlreadyInProgress = errors.New("an operation is already in progress")

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
	UpgradePackages(ctx context.Context, names []string) (<-chan Event, error)
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
		return nil, errors.New("no internet connectivity")
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
		arduinoEvents, err := m.arduinoPlatformUpdateService.UpgradePackages(ctx, arduinoPlatform)
		if err != nil {
			m.broadcast(
				Event{
					Type: ErrorEvent,
					Data: "failed to upgrade Arduino packages",
					Err:  err,
				})
			return
		}
		for e := range arduinoEvents {
			m.broadcast(e)
		}

		aptEvents, err := m.debUpdateService.UpgradePackages(ctx, debPkgs)
		if err != nil {
			m.broadcast(
				Event{
					Type: ErrorEvent,
					Data: "failed to upgrade APT packages",
					Err:  err,
				})
			return
		}
		for e := range aptEvents {
			m.broadcast(e)
		}
		m.broadcast(Event{Type: DoneEvent, Data: "Upgrade completed successfully"})
	}()
	return nil
}

// Subscribe creates a new channel for receiving APT events.
func (b *Manager) Subscribe() chan Event {
	eventCh := make(chan Event, 100)
	b.mu.Lock()
	b.subs[eventCh] = struct{}{}
	b.mu.Unlock()
	return eventCh
}

// Unsubscribe removes the channel from the list of subscribers and closes it.
func (b *Manager) Unsubscribe(eventCh chan Event) {
	b.mu.Lock()
	delete(b.subs, eventCh)
	close(eventCh)
	b.mu.Unlock()
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
				slog.String("data", fmt.Sprintf("%v", event.Data)),
				slog.Any("error", event.Err),
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
