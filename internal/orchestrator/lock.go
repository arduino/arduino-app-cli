// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/arduino/go-paths-helper"
	"golang.org/x/sync/singleflight"
	"golang.org/x/sys/unix"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

const (
	appsLockFileName = "apps.lock"
	lockPollInterval = 50 * time.Millisecond
)

// runExclusive runs fn, the operation kind of app, one app operation at a time on the board,
// CLI and daemon included. Equal requests in flight, same app and same operation, share one
// run and its result. Not reentrant: fn calls the unlocked variants of the other operations.
func runExclusive(ctx context.Context, cfg config.Configuration, app app.ArduinoApp, kind string, fn func() error) error {
	key := app.FullPath.String() + " " + kind
	for {
		_, err, shared := appRequests.Do(key, func() (any, error) {
			err := withAppsLock(ctx, cfg, fn)
			if err != nil && ctx.Err() != nil {
				return nil, canceledRun{err}
			}
			return nil, err
		})
		if shared {
			slog.Info("the request shared the run of an equal one", slog.String("app", key))
		}
		var canceled canceledRun
		if !errors.As(err, &canceled) {
			return err
		}
		// The run ended with the cancellation of the request that made it, which says
		// nothing about the operation: a request still alive runs it.
		if ctx.Err() != nil {
			return canceled.err
		}
	}
}

// appRequests merges the equal requests in flight in this process: the ones after the first
// wait for its run and get its result, a panic included.
var appRequests singleflight.Group

// canceledRun is the error of a run cut short by the cancellation of its own request.
type canceledRun struct{ err error }

func (c canceledRun) Error() string { return c.err.Error() }
func (c canceledRun) Unwrap() error { return c.err }

// withAppsLock runs fn holding the lock that serializes the app operations of the CLI and the
// daemon. A process that dies releases it: the kernel holds the lock, the file is only its name.
func withAppsLock(ctx context.Context, cfg config.Configuration, fn func() error) error {
	unlock, err := lockFile(ctx, cfg.DataDir().Join(appsLockFileName))
	if err != nil {
		return err
	}
	defer unlock()
	return fn()
}

// lockFile takes an exclusive flock on path, polling so that ctx can cancel the wait.
func lockFile(ctx context.Context, path *paths.Path) (func(), error) {
	if err := path.Parent().MkdirAll(); err != nil {
		return nil, fmt.Errorf("failed to create the lock folder: %w", err)
	}
	// Read only: taking the lock needs no write access to the file.
	file, err := os.OpenFile(path.String(), os.O_RDONLY|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open the lock file: %w", err)
	}
	fd := int(file.Fd()) //nolint:gosec // a file descriptor fits an int
	waiting := false
	for {
		err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return func() {
				_ = unix.Flock(fd, unix.LOCK_UN)
				_ = file.Close()
			}, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) {
			_ = file.Close()
			return nil, fmt.Errorf("failed to lock %s: %w", path, err)
		}
		if !waiting {
			slog.Info("waiting for another app operation to finish", slog.String("lock", path.String()))
			waiting = true
		}
		select {
		case <-ctx.Done():
			_ = file.Close()
			return nil, ctx.Err()
		case <-time.After(lockPollInterval):
		}
	}
}
